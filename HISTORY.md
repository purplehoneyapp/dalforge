# Version History

## v1.3.3 - 2026-03-27
  - fixed dsn connection string to use &loc=UTC for db connections.
  - fixed infof to debugf for cache invalidations.
  - configuration for server group support "all" value to allow any entity name.
  - increased length of uid from 50 to 255 chars
  - fixed bug in createtable when running sql statements with comments 
  - closing redis will nicely close pubsub connections, too.
  - circuit breaker now ignores ErrNotFound error type

## v1.3.0 - 2026-03-17
  - added support for json field type

## v1.2.0 - 2026-03-09
  - added typeMapping instructions for where clauses. so definig where: user1 = :user OR user2 = :user is possible

## v1.1.0 - 2026-02-24
 - renamed DAL to Repository structs
 - improved various small bugs


## v1.0.0 - first release