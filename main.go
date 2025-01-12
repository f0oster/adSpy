package main

import (
	"context"

	"f0oster/adspy/activedirectory"
	"f0oster/adspy/activedirectory/ldaphelpers"
	"f0oster/adspy/config"
	"f0oster/adspy/database"
)

func main() {

	ctx := context.Background()
	database.ResetDatabase(ctx)

	adSpyConfig := config.LoadEnvConfig("settings.env")
	adInstance := activedirectory.NewActiveDirectoryInstance(adSpyConfig.BaseDN, adSpyConfig.DcFQDN, adSpyConfig.PageSize)
	adInstance.Connect(adSpyConfig.Username, adSpyConfig.Password)
	adInstance.LoadSchema()
	adInstance.FetchHighestUSN()
	adInstance.FetchPagedEntriesWithCallback(ldaphelpers.AllGroupObjects, 1000, ldaphelpers.PrintToConsole)

	return
}
