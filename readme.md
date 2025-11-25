# adSpy

**adSpy** is an open-source Active Directory change auditing tool written in Go. It is currently just a proof of concept and is **not intended or ready for production use**.

The tool monitors Active Directory for object-level changes, snapshots those changes, and stores them in a database for later analysis. It is designed to operate with *least-privilege* access and aims to be fully cross-platform, though testing so far has been limited to Windows.

## How It Works

- On the initial run, adSpy enumerates the domain and creates a versioned snapshot for each directory object.
- A polling loop issues an LDAP query for objects with an incremented **uSNChanged** value.  
  Microsoft overview: https://learn.microsoft.com/en-us/windows/win32/ad/polling-for-changes-using-usnchanged
- When an object changes, adSpy:
  - Detects and stores the specific attribute differences  
  - Stores a new object snapshot
  - Preserves all historical change units for that object  

## Long-Term Goals

- Support for Kerberos authentication, LDAPS, channel binding, and other basic security features  
- Parsing of security descriptor changes
- Correlate changes back to the security principal that performed them  
- Support for database drivers other than PostgreSQL  
- Some sort of interface to monitor or review changes (likely a web app)

## Disclaimers

- **State persistence is not yet implemented.**  
  On startup, adSpy always drops and recreates its database, rebuilding it from the live directory - as above, the project is not intended to be used for real environments yet.

- **Secure authentication is not yet implemented.**  
  The current LDAP bind / authentication method is suitable for development/testing only.  
  Do *not* run this tool in a real production Active Directory environment.

## Installation

```bash
git clone https://github.com/f0oster/adSpy
cd adSpy
go build -o adSpy
./adSpy
```

## Configuration

Configure **adSpy** via the `settings.env` file. Note that adSpy will require a PostgreSQL instance to be set up and configured.

Example `settings.env`:

```env
LDAP_BASE_DN="dc=example,dc=com"
LDAP_DCFQDN="lab.dc.com"
LDAP_USERNAME="svc-ldap@lab.dc.com"
LDAP_PASSWORD="password"
LDAP_PAGESIZE=1000
DB_MANAGEMENT_DSN=postgres://postgres:example@localhost:5432/postgres
DB_ADSPY_DSN=postgres://postgres:example@localhost:5432/adspy
```

## Service Account Permissions

To monitor changes to objects in Active Directory, the service account needs read access to all of the objects that you intend to monitor for changes. By default, read permissions on most directory objects are already granted to `Authenticated Users` via membership to the `BUILTIN\Pre-Windows 2000 Compatible Access` security group. Some organizations rightfully choose to remove `Authenticated Users` from this group when hardening their environment to make directory reconnisaince and enumeration more challenging. In these cases, the simplest way to get up and running (and what I'd likely do) is to add the service account as a member of `BUILTIN\Pre-Windows 2000 Compatible Access` security group, but you should use your own judgement here - if appropriate, you can delegate more granular read permissions for the service account in line with your security posture.

To detect object deletions in Active Directory, the service account needs read access to the Deleted Objects container. By default, only privileged users and groups hold permissions to query this container.

Per [Microsoft documentation](https://learn.microsoft.com/en-us/troubleshoot/windows-server/active-directory/non-administrators-view-deleted-object-container):

> To grant a user or service account access to view the Deleted Objects container:
>
> 1. Take ownership of the container:
>    ```
>    dsacls "CN=Deleted Objects,DC=YourDomain,DC=com" /takeownership
>    ```
>
> 2. Grant the user permissions:
>    ```
>    dsacls "CN=Deleted Objects,DC=YourDomain,DC=com" /g DOMAIN\Username:LCRP
>    ```
>
> The LIST CONTENTS and READ PROPERTY permissions let the user view the contents of the deleted objects container without making changes.

Replace `DC=YourDomain,DC=com` with your domain structure and `DOMAIN\Username` with your service account. Generally, it is a better practice to delegate permissions to security groups rather than users directly, so if you prefer to do that instead, that is a valid approach.
