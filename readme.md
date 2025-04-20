# adSpy

**adSpy** is an (unfinished, and not yet functional) open-source Active Directory change auditing tool written in Go.

The goal of **adSpy** is to provide near real-time visibility into changes made to your Active Directory domain as they happen, as well as track historical changes over time, helping you monitor and audit modifications such as user, group, and organizational unit changes. 

## Planned / Future Features

- **Near Real-time Object Monitoring**: Receive updates as changes are made or replicated within Active Directory
- **Audit & Logging**: Keep track of all historical changes for compliance or troubleshooting
- **Efficient Processing**: Written in Go with the hopes of having decent performance and scalability.

## Installation

To install **adSpy**, follow these steps:

1. Clone the repository:
    ```bash
    git clone https://github.com/f0oster/adSpy
    ```

2. Navigate into the project directory:
    ```bash
    cd adSpy
    ```

3. Build the project:
    ```bash
    go build -o adSpy
    ```

4. Run the tool:
    ```bash
    ./adSpy
    ```

## Configuration

You can configure **adSpy** via the `settings.env` file.

Example `settings.env`:
```env
LDAP_BASE_DN="dc=example,dc=com"
LDAP_DCFQDN="lab.dc.com"
LDAP_USERNAME="svc-ldap@lab.dc.com"
LDAP_PASSWORD="password"
LDAP_PAGESIZE=1000
```
