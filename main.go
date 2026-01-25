package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"f0oster/adspy/activedirectory"
	"f0oster/adspy/activedirectory/ldaphelpers"
	"f0oster/adspy/config"
	"f0oster/adspy/database"
	"f0oster/adspy/snapshot"
	"f0oster/adspy/versioning"

	"github.com/go-ldap/ldap/v3"
)

func main() {
	adSpyConfig := config.LoadEnvConfig("settings.env")

	ctx := context.Background()
	database.ResetDatabase(ctx, adSpyConfig.ManagementDsn, adSpyConfig.AdSpyDsn)
	db := database.NewDatabase(adSpyConfig.AdSpyDsn, adSpyConfig.ManagementDsn)
	if err := db.Connect(ctx); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	adInstance, err := activedirectory.NewActiveDirectoryInstance(adSpyConfig)
	if err != nil {
		log.Fatalf("failed to initialize Active Directory instance: %v", err)
	}

	err = db.Client().InsertDomain(
		ctx,
		adInstance.DomainId,
		adInstance.BaseDn,
		adInstance.DomainControllerFQDN,
		adInstance.HighestCommittedUSN,
	)
	if err != nil {
		log.Fatalf("failed to insert domain entry to database: %v", err)
	}

	// Persist AD schema to database
	schemas := adInstance.SchemaRegistry.GetAllSchemas()
	for _, s := range schemas {
		if err := db.Client().UpsertAttributeSchema(
			ctx,
			s.ObjectGUID,
			adInstance.DomainId,
			s.AttributeLDAPName,
			s.AttributeName,
			s.AttributeID,
			s.AttributeSyntax,
			s.AttributeOMSyntax,
			s.AttributeFieldType.SyntaxName,
			s.AttributeIsSingleValued,
		); err != nil {
			log.Fatalf("failed to persist attribute schema: %v", err)
		}
	}
	log.Printf("Persisted %d attribute schemas", len(schemas))

	snapshotService := snapshot.NewService()
	versioningService := versioning.NewService(db.Client(), snapshotService, adInstance.DomainId, adInstance.SchemaRegistry)

	log.Println("adSpy initialized - monitoring AD for changes")

	for {
		if err := processChanges(ctx, adInstance, snapshotService, versioningService); err != nil {
			log.Printf("Error processing changes: %v", err)
		}

		if err := adInstance.FetchHighestUSN(); err != nil {
			log.Printf("Error fetching highest USN: %v", err)
		}

		time.Sleep(1 * time.Second)
	}
}

// processChanges fetches LDAP entries, parses them, creates snapshots, and persists to database.
func processChanges(
	ctx context.Context,
	adInstance *activedirectory.ActiveDirectoryInstance,
	snapshotService *snapshot.Service,
	versioningService *versioning.Service,
) error {
	// Note: LDAP doesn't support > operator, so we use >= with (USN + 1)
	ldapFilter := ldaphelpers.And(
		ldaphelpers.Or(
			ldaphelpers.Eq("objectCategory", "*"), // Live objects
			ldaphelpers.Eq("isDeleted", "TRUE"),   // Deleted objects
		),
		ldaphelpers.Ge("uSNChanged", adInstance.HighestCommittedUSN+1),
	).String()

	var allEntries []*ldap.Entry
	err := adInstance.ForEachLDAPPage(ldapFilter, 1000,
		func(ad *activedirectory.ActiveDirectoryInstance, entries []*ldap.Entry) error {
			allEntries = append(allEntries, entries...)
			return nil
		})
	if err != nil {
		return fmt.Errorf("LDAP query failed: %w", err)
	}

	if len(allEntries) == 0 {
		// No changes - this is normal
		return nil
	}

	log.Printf("Fetched %d entries from LDAP", len(allEntries))

	parser := activedirectory.NewParser(adInstance.SchemaRegistry)
	results := parser.ParseEntries(allEntries)

	var objects []*activedirectory.ActiveDirectoryObject
	var parseErrors int
	for _, result := range results {
		if result.Error != nil {
			log.Printf("Failed to parse entry %s: %v", result.DN, result.Error)
			parseErrors++
		} else {
			objects = append(objects, result.Object)
		}
	}

	if parseErrors > 0 {
		log.Printf("Warning: %d entries failed to parse", parseErrors)
	}

	if len(objects) == 0 {
		log.Println("No objects to process after parsing")
		return nil
	}

	log.Printf("Successfully parsed %d objects", len(objects))

	snapshots := make([]*snapshot.Snapshot, 0, len(objects))
	var snapshotErrors int
	for _, obj := range objects {
		snap, err := snapshotService.CreateSnapshot(obj)
		if err != nil {
			log.Printf("Failed to create snapshot for %s: %v", obj.DN, err)
			snapshotErrors++
			continue
		}
		snapshots = append(snapshots, snap)
	}

	if snapshotErrors > 0 {
		log.Printf("Warning: %d snapshots failed to create", snapshotErrors)
	}

	if len(snapshots) == 0 {
		log.Println("No snapshots to save")
		return nil
	}

	log.Printf("Created %d snapshots", len(snapshots))

	// Fail-fast transaction
	if err := versioningService.ProcessSnapshots(ctx, snapshots, adInstance.DomainId); err != nil {
		return fmt.Errorf("failed to process snapshots: %w", err)
	}

	log.Printf("Successfully processed %d objects", len(snapshots))
	return nil
}
