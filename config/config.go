package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type ADSpyConfiguration struct {
	BaseDN   string
	DcFQDN   string
	Username string
	Password string
	PageSize uint32
}

func LoadEnvConfig(configName string) ADSpyConfiguration {
	err := godotenv.Load(configName)
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	baseDN := string(os.Getenv("LDAP_BASEDN"))
	dcFQDN := string(os.Getenv("LDAP_DCFQDN"))
	username := string(os.Getenv("LDAP_USERNAME"))
	password := string(os.Getenv("LDAP_PASSWORD"))
	tempPageSize, err := strconv.Atoi(os.Getenv("LDAP_PAGESIZE"))

	if err != nil {
		log.Fatalf("failed to parse integer: %v", err)
	}

	pageSize := uint32(tempPageSize)

	return ADSpyConfiguration{
		BaseDN:   baseDN,
		DcFQDN:   dcFQDN,
		Username: username,
		Password: password,
		PageSize: pageSize,
	}

}
