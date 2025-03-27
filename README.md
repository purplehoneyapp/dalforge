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

<inputdir>: Directory containing your YAML files.
<outputdir>: Directory where the generated DAL code will be written.

## Example
Suppose you have a folder ./config with your YAML files and you want to generate the DAL code in the folder ./dal. Run:

```sh
dalforge generate ./dalconfig ./dal
```

YAML Configuration
Each YAML file defines an entity and its fields. For example:

yaml
Copy
# user.yaml
entity: user
fields:
  id:
    type: int64
    primary: true
  age:
    type: int8
    allowNull: false
  email:
    type: string
    unique: true
  birthdate:
    type: date
  created:
    type: timestamp
    default: CURRENT_TIMESTAMP
  updated:
    type: timestamp
    default: CURRENT_TIMESTAMP
    onUpdate: CURRENT_TIMESTAMP
DALForge uses these definitions to generate Go files (e.g. user.gen.go) that implement a complete DAL layer with caching, telemetry, and error handling.

## Configuration & Customization
DALForge comes with sensible defaults. You can customize various aspects:

Caching:
Generated code supports in-memory caching and optional Redis-based cache invalidation. Adjust connection settings in the generated files if needed.

Circuit Breaker:
Uses gobreaker with default thresholds that you can tune in the generated code.

Telemetry:
Metrics are collected via Prometheus. You can customize which metrics are exposed or how they’re reset between test runs.

## Testing
DALForge is fully tested. To run tests, execute:

sh
Copy
go test ./...
Note:
Global Prometheus counters are shared across tests. Use the provided telemetry reset functions (unregisterTelemetry() and registerTelemetry()) in your test setup if you require isolated metric state.

## Examples
Generating a DAL for a User Entity
Create a YAML file (e.g., user.yaml) in your configuration directory (./config):

yaml
Copy
entity: user
fields:
  id:
    type: int64
    primary: true
  age:
    type: int8
    allowNull: false
  email:
    type: string
    unique: true
  birthdate:
    type: date
  created:
    type: timestamp
    default: CURRENT_TIMESTAMP
  updated:
    type: timestamp
    default: CURRENT_TIMESTAMP
    onUpdate: CURRENT_TIMESTAMP
Run DALForge to generate the DAL code:

sh
Copy
dalforge generate ./config ./dal
The generated code in ./dal will include a file (e.g., user.gen.go) implementing a DAL layer for the user entity with full CRUD, caching, telemetry, and error handling.

## Contributing
Contributions are welcome! Please fork the repository, commit your changes, and open a pull request. For major changes, open an issue first to discuss your ideas.

## License
DALForge is released under the MIT License. See the LICENSE file for details.

## Contact
For questions, issues, or feature requests, please open an issue on the GitHub repository.
