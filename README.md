# DoctorGPT
DoctorGPT brings GPT into production for error diagnosing!
(Not production ready, yet...)

DoctorGPT is a lightweight self-contained binary that monitors your application logs for problems and diagnoses them.

## Usage
`OPENAI_KEY=$YOUR_KEY doctorgpt --logfile="program.log" --configfile="config.yaml" --outdir="~/errors"`

DoctorGPT will start tailing `program.log` (without stopping). For each log line, user-defined parsers triggering a diagnosis event (based on regex variable matches) will generate a diagnosis file (see example below) under directory `~/errors` using the triggered log line and all previous log context using the OpenAI API. `config.yaml` file is used at startup to configure the program.

## CLI flags
- `--logfile (string)` log file to tail and monitor
- `--configfile (string)` yaml config file location
- `--outdir (string)` diagnosis files directory (created if it does not exist)
- `--bundlingtimeoutseconds (int)` wait some time for logs to come-in after the triggered line (for multi-line error dumps) (`default: 5`)
- `--debug (bool)` debug logging (`default: true`)
- `--buffersize (int)` maximum number of log entries per buffer  (`default: 100`)
- `--maxtokens (int)` maximum number of tokens allowed in API (`default: 8000`)
- `--gptmodel (string)` GPT model to use (`default: "gpt-4"`). For list of models see: [OpenAI API Models](https://platform.openai.com/docs/models/overview)

## Configuration
See example yaml documentation:
```yaml
# Prompt to be sent alongside error context to the GPT API
prompt: "You are ErrorDebuggingGPT. Your sole purpose in this world is to help software engineers by diagnosing software system errors and bugs that can occur in any type of computer system. The message following the first line containing \"ERROR:\" up until the end of the prompt is a computer error no more and no less. It is your job to try to diagnose and fix what went wrong. Ready?\nERROR:\n$ERROR"

parsers:

  # Matches line: [1217/201832.950515:ERROR:cache_util.cc(140)] Unable to move cache folder GPUCache to old_GPUCache_000
  - regex: '^\[(\d{4}\/\d{6}\.\d{6}):(?P<LEVEL>\w+):([\w\.\_]+)\(\d+\)\]\s+(?P<MESSAGE>.*)$'

    # Conditions in which the parsed log will trigger a diagnosis
    triggers:
      - variable: "LEVEL"
        regex:    "ERROR"

    # Conditions in which the parsed log will be ignored for triggers
    # To create exceptions which won't trigger the GPT API
    filters:
      - variable: "MESSAGE"
        regex:    "HTTP 401"

    # Conditions in which the parsed log will be ignored and excluded from the API context
    # For sensitive or spammy log entries. These will never be sent to the GPT API
    excludes:
      - variable: "LEVEL"
        regex:    "DEBUG"

  # Matches line:  2022-01-27 21:37:36.776 0x2eb3     Default       511 photolibraryd: PLModelMigration.m:314   Creating sqlite error indicator file
  - regex: '^(?P<DATE>[^ ]+)\s+(?P<TIME>[^ ]+)\s+[^ ]+(?P<LEVEL>[^ ]+)\s+(?P<MESSAGE>.*)$'

  # When more than one trigger is present, just one trigger is sufficient to trigger a diagnosis
    triggers:
      - variable: "LEVEL"
        regex:    "Default"
      - variable: "MESSAGE"
        regex:    "(?i)ERROR:"
    # Filters and excludes were not specified

  # Last parser must always be a generic one that matches any line
  - regex: '^(?P<MESSAGE>.*)$'
    # All filters, triggers and excludes were not specified
```

## Example
This is how the file `::Users::yamilmolinar::error.log:18.diagnosed` file looks like:
```
LOG LINE:
/Users/yamilmolinar/error.log:18

BASE PROMPT:
 You are ErrorDebuggingGPT. Your sole purpose in this world is to help software engineers by diagnosing software system errors and bugs that can occur in any type of computer system. The message following the first line containing \"ERROR:\" up until the end of the prompt is a computer error no more and no less. It is your job to try to diagnose and fix what went wrong. Ready?\nERROR:\n$ERROR

CONTEXT:
yarn run v1.22.19
$ tsnd --respawn --transpile-only --no-notify --ignore-watch node_modules src/index.ts
[INFO] 15:20:25 ts-node-dev ver. 2.0.0 (using ts-node ver. 10.8.0, typescript ver. 4.8.4)
[INFO]  DB ready
[INFO]  Auth ready
[INFO]  Apollo setup
[INFO]  Server started at http://localhost:5555/graphql 🚀

Query: Me
POST /graphql 200 241.389 ms - 21

prisma:query SELECT 1
prisma:query SELECT "public"."User"."id", "public"."User"."email", "public"."User"."password", "public"."User"."firstName", "public"."User"."lastName", "public"."User"."avatar", "public"."User"."role", "public"."User"."bio", "public"."User"."createdAt", "public"."User"."updatedAt" FROM "public"."User" WHERE "public"."User"."email" = $2 LIMIT $2 OFFSET $3
[ERROR]  PrismaClientKnownRequestError:
Invalid `prisma.user.findFirst()` invocation in
/Users/yamilmolinar/Repos/boilerplate/packages/api/src/modules/user/user.service.ts:32:36

  29 }
  30
  31 async checkUserExists(where: UserWhereInput) {
→ 32   const user = await prisma.user.findFirst(
The table `public.User` does not exist in the current database.
    at RequestHandler.handleRequestError (/Users/yamilmolinar/Repos/boilerplate/node_modules/@prisma/client/runtime/index.js:30851:13)
    at RequestHandler.request (/Users/yamilmolinar/Repos/boilerplate/node_modules/@prisma/client/runtime/index.js:30834:12)
    at PrismaClient._request (/Users/yamilmolinar/Repos/boilerplate/node_modules/@prisma/client/runtime/index.js:31812:16)
    at UserService.checkUserExists (/Users/yamilmolinar/Repos/boilerplate/packages/api/src/modules/user/user.service.ts:32:18)
    at UserService.register (/Users/yamilmolinar/Repos/boilerplate/packages/api/src/modules/user/user.service.ts:26:5)
    at UserResolver.register (/Users/yamilmolinar/Repos/boilerplate/packages/api/src/modules/user/user.resolver.ts:93:18)
    at dispatchHandler (/Users/yamilmolinar/Repos/boilerplate/node_modules/type-graphql/dist/resolvers/helpers.js:82:24)
    at /Users/yamilmolinar/Repos/boilerplate/node_modules/type-graphql/dist/resolvers/helpers.js:83:26
    at ErrorInterceptor (/Users/yamilmolinar/Repos/boilerplate/packages/api/src/lib/globalMiddleware.ts:29:12)
    at dispatchHandler (/Users/yamilmolinar/Repos/boilerplate/node_modules/type-graphql/dist/resolvers/helpers.js:82:24) {
  code: 'P2021',
  clientVersion: '4.4.0',
  meta: { table: 'public.User' }
}

Mutation: Register
POST /graphql 200 365.616 ms - 2139

DIAGNOSIS:
It seems the error is caused by a missing table in the database. The table `public.User` does not exist in the current database, and this is causing the `PrismaClientKnownRequestError` to be thrown when trying to execute `prisma.user.findFirst()` in the `UserService.checkUserExists()` function.

To resolve this issue, follow these steps:

1. Ensure that you have properly set up your database schema and that the `public.User` table is defined. If it's not, add the necessary schema definition for the User table.

2. Ensure that Prisma is properly configured to connect to your database. Double-check the configuration in your `prisma/schema.prisma` file, and make sure the database URL and other settings are correct.

3. Run a migration to update your actual database's schema. You can do this by running the following command:

npx prisma migrate dev --name init --preview-feature

This command will create the necessary tables in your database based on your Prisma schema file, apply the migration, and generate the Prisma Client.

4. After running the migration, restart your server and test your application again to ensure that the error is resolved.

If you still encounter issues, double-check your Prisma configuration, as well as your database connection settings, and ensure your code logic is correct.
```

## Parser library (to be enhanced)
A library of common log parsers is contained in `config.yaml`. These parsers were tested against real logs in `testlogs/*_2k.log`. 

1. Android
2. Apache
3. HDFS
4. Hadoop
5. Linux
6. Mac
7. Spark
8. Windows

See `parsers_test.go` for tests and [loghub](https://github.com/logpai/loghub) for more log samples.

## Installation
Using `go install`:
- `GOBIN=/usr/local/bin go install "github.com/ingyamilmolinar/doctorgpt/agent"`

## Dependencies
1. A `Go` compiler (for building and running tests only)
2. `docker`  (for development)
3. `k3d`     (for development)
4. `kubectl` (for development)
5. `make`    (for development)

## Features (to be enhanced)
1. Environment independent self-sufficient lightweight (8.3MB) binary. (Windows support is missing but could be easily added)
2. Alpine Linux Docker image
3. Configurable chatGPT prompt
4. Supports every GPT model version
5. Match multiple log formats within the same file
6. Match multiple parsers for the same log entry
7. Match multiple filters for the same log entry
8. Ignore logs from being part of the context (for sensitive or spammy information)
9. Support custom variable names
10. Powerful regex format (Perl/Go flavor)
11. Maximize the amount of log context in the diagnosis

## Work in progress
1. Dividing log contexts per custom regex match in variable values
2. Enhance library of common log parsers

## Future work
1. Structured logging parsing
2. Generate a config.yaml based on real life log examples (boottrap config using GPT)
3. "FROM scratch" lightweight docker image
4. Release strategy & CI
5. Windows / Mac support
6. Support custom types (for timestamp comparisons, etc)
7. Production readiness (security, auth, monitoring, optimization, more tests...)
8. Sentry SDK integration
9. Helm chart

## Development
- `export OPENAI_API=<your-api-key>`
- `make k4d-create` (to create k3d cluster)
- `make k3d-delete` (to delete k3d cluster)
NOTE: See `Makefile` for more commands

## Testing (Tests do not use OpenAI API)
- `cd agent; go test ./...; cd -`
- `cd agent; go test -v ./...; cd -` (verbose mode)

## Contributing
Feel free to open an issue with your suggestion on how to make this program more useful, portable, efficient and production-ready (and of course BUGS!).

Feel free to open MRs. I'll review them if I see they follow the philosophy of this project. For larger chunks of work or design changes, please open an issue first so that the strategy can be discussed.
