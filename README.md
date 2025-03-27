# DALForge

DALForge is a CLI tool that generates a complete Data Access Layer (DAL) from YAML configuration files. It streamlines the process of building standardized, production-ready DAL code for your Go projects by transforming simple YAML definitions into robust, well-organized Go code.

## Features

- **Automatic Code Generation:**
  Generate DAL code (CRUD operations, caching, telemetry, circuit breaker integration, etc.) directly from YAML definitions.

- **Customization:**
  Define your entities, fields, and relationships in YAML, and let DALForge create the corresponding DAL layer.

- **Built-In Best Practices:**
  The generated code includes support for:
  - Caching with both local in-memory (go-cache) and distributed invalidation via Redis pub/sub.
  - Circuit breaker integration using [gobreaker](https://github.com/sony/gobreaker).
  - Telemetry with Prometheus.

- **Open Source:**
  DALForge is fully open source. Contributions, issues, and feature requests are welcome!

## Installation

1. **Prerequisites:**
   - [Go](https://golang.org/doc/install) (version 1.16 or later)
   - Redis (if you plan to use Redis for distributed cache invalidation)
   - YAML configuration files defining your entities

2. **Install via `go install`:**

   ```sh
   go install github.com/purplehoneyapp/dalforge@latest


## Usage
DALForge is a command-line tool that generates your DAL code based on YAML configuration files.

## Command Syntax
```sh
dalforge generate <inputdir> <outputdir>
```

inputdir: Directory containing your YAML files.
outputdir: Directory where the generated DAL code will be written.

## Example
Suppose you have a folder ./config with your YAML files and you want to generate the DAL code in the folder ./dal. Run:

```sh
dalforge generate ./dalconfig ./dal
```

YAML Configuration
Each YAML file defines an entity and its fields. For example:

```yaml
name: user  # entity name, should be singular, snake cased.
version: v1
columns:
  status:
    type: string # supported types are: int8, int32, int64, bool, string, float, date, time, datetime, uuid
    allowNull: true
  uuid:
    type: uuid
    unique: true
  email:
    type: string
    allowNull: false
    unique: true
  birthdate:
    type: date
    allowNull: true
  age:
    type: int8
operations:
  store: true   # if true will generate proper write functions
  delete: true  # if true it will generate delete functions
  gets:
    - email   # put a column name here; it will generate GetByEmail function. The intention is that those columns are unique values.
    - uuid
  lists:
    - name: list_by_id                 # name of a function that will fetch multiple rows with pagination supported. This should be snake cased and have a name that sounds like multiple results are expected.
    - name: list_by_bday
      where: birthdate < :birthdate    # you can use named parameters. This will create a ListByBDay(ctx, birthdate time.Time, startId int64, pageSize int) function
      order: birthdate
      desc: true
    - name: list_by_age
      where: age = :age
      order: created
     # desc: true   # default is false
    - name: list_by_status
      where: status = :status
      order: created
circuitbreaker:
  timeoutSeconds: 20             # how long to wait while in open state before trying to go to half open state.
  consecutiveFailures: 4         # how many times to fail in an closed state before we swithc to open state
caching:
  type: redis # Potential values could be: none, redis. If none that no cache invalidation across multiple instances would happen. Most likely you would have redis here.
  singleExpirationSeconds: 300  # timeout for local cache for single rows
  listExpirationSeconds: 60     # timeout for local cache for multiple rows aka list functions
  listInvalidation: flush       # potential values could be: expire, flush (default is flush); expire means list cache will only expire. flush means that on any change of data list cache will be flushed.
  maxItemsCount: 100000         # max number of items in cache
```

DALForge uses these definitions to generate Go files (e.g. user.gen.go) that implement a complete DAL layer with caching, telemetry, and error handling.

## Configuration & Customization
DALForge comes with sensible defaults. You can customize various aspects:

**Configuration for MYSQL:**
To define MYSQL servers check this configuration file: [serverprovider.yaml](example/dal/serverprovider.yaml)

**Caching:**
Generated code supports in-memory caching and optional Redis-based cache invalidation. Adjust connection settings in the generated files if needed.

**Circuit Breaker:**
Uses gobreaker with default thresholds that you can tune in the generated code.

**Telemetry:**
Metrics are collected via Prometheus.

## Testing
DALForge is fully tested. To run tests, execute:

```sh
go test ./...
```

Generated code is tested in this repository so there is no really need to add unit tests to the generated DAL layer code in your real repositories.

Note:
Global Prometheus counters are shared across tests. Use the provided telemetry reset functions (unregisterTelemetry() and registerTelemetry()) in your test setup if you require isolated metric state.

## Examples
Check examples directory for yaml example and generated code examples.

## Contributing
Contributions are welcome! Please fork the repository, commit your changes, and open a pull request. For major changes, open an issue first to discuss your ideas.

## License
DALForge is released under the MIT License. See the LICENSE file for details.

## Contact
For questions, issues, or feature requests, please open an issue on the GitHub repository.
