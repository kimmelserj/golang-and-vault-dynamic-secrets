path "database/creds/app" {
  capabilities = [ "read" ]
}

path "sys/leases/renew" {
  capabilities = [ "update" ]
}

path "sys/leases/revoke/database/creds/app/*" {
  capabilities = [ "update" ]
}