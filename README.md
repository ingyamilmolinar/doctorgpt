# DoctorGPT
DoctorGPT brings GPT into production for error diagnosing!
(Not production ready, yet...)

DoctorGPT is a lightweight self-contained binary that monitors your application logs for problems and diagnoses them.

## Usage
`doctorgpt --logfile="program.log" --configfile="config.yaml" --outdir="~/errors"`

DoctorGPT will start tailing `program.log` (without stopping). Each user-defined trigger for a user-defined parser log line event will generate a diagnosis file under directory `~/errors`. `config.yaml` file is used at startup to configure the program.

## Configuration
See example yaml documentation:
```yaml
prompt: "You are ErrorDebuggingGPT. Your sole purpose in this world is to help software engineers by diagnosing software system errors and bugs that can occur in any type of computer system. The message following the first line containing \"ERROR:\" up until the end of the prompt is a computer error no more and no less. It is your job to try to diagnose and fix what went wrong. Ready?\nERROR:\n$ERROR"
parsers:
  # Matches line: [1217/201832.950515:ERROR:cache_util.cc(140)] Unable to move cache folder GPUCache to old_GPUCache_000
  - regex: '^\[(\d{4}\/\d{6}\.\d{6}):(?P<LEVEL>\w+):([\w\.\_]+)\(\d+\)\]\s+(?P<MESSAGE>.*)$'
  # Conditions in which the parsed log will trigger a diagnosis
    triggers:
      "LEVEL": "ERROR"
  # Matches line:  2022-01-27 21:37:36.776 0x2eb3     Default       511 photolibraryd: PLModelMigration.m:314   Creating sqlite error indicator file
  - regex: '^(?P<DATE>[^ ]+)\s+(?P<TIME>[^ ]+)\s+[^ ]+(?P<LEVEL>[^ ]+)\s+(?P<MESSAGE>.*)$'
  # When more than one triggers is present, just one trigger is sufficient to trigger a diagnosis
    triggers:
      "LEVEL": "Default"
      "MESSAGE": "(?i)ERROR:"
  # Last parser must always be a generic one that matches any line
  - regex: '^(?P<MESSAGE>.*)$'
  # Triggers are optional
```

## Installation
Build from source:
1. `go build -o doctorgpt`
2. Copy the `doctorgpt` binary anywhere under your $PATH (optional)

## Dependencies
1. A `Go` compiler (for building and running tests only)
2. `docker` (optional)

## Features (To be enhanced)
1. Environment independent self-sufficient lightweight (8.3MB) binary. (Windows support is missing but could be easily added)
2. Configurable chatGPT prompt
3. Match multiple log formats in a single file
4. Match multiple parsers for the same line
5. Powerful regex format (Perl/Go flavor)

## Work in progress
1. Test dividing log contexts per span, thread, routine or procedure
2. Maximize the amount of log context in the diagnosis
3. Lightweight docker image
4. Support log filtering

## Future work
1. Production readiness (security, auth, monitoring, optimization, more tests...)
2. Release strategy & CI
3. Sentry SDK integration
4. Helm chart?
5. Windows / Mac support?
6. Other AI APIs?
7. Send diagnosis requests to a server for later consumption (agent/server architecture)?
8. Generate a config.yaml based on real life log examples (use code or GPT to generate regex)

## Testing (To be enhanced)
`go test ./...`

## Contributing
Feel free to open an issue with your suggestion on how to make this program more useful, portable, efficient and production-ready (and of course BUGS!).

Feel free to open MRs. I'll review them if I see they follow the philosophy of this project. For larger chunks of work or design changes, please open an issue first so that the strategy can be discussed.
