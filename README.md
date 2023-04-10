# DoctorGPT
DoctorGPT brings GPT into production for error diagnosing!
(Not production ready, yet...)

DoctorGPT is a lightweight self-contained binary that monitors your application logs for problems and diagnoses them.

## Usage
`OPENAI_KEY=$YOUR_KEY doctorgpt --logfile="program.log" --configfile="config.yaml" --outdir="~/errors"`

DoctorGPT will start tailing `program.log` (without stopping). Each user-defined trigger for a user-defined parser log line event will generate a diagnosis file under directory `~/errors`. `config.yaml` file is used at startup to configure the program.

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
prompt: "You are ErrorDebuggingGPT. Your sole purpose in this world is to help software engineers by diagnosing software system errors and bugs that can occur in any type of computer system. The message following the first line containing \"ERROR:\" up until the end of the prompt is a computer error no more and no less. It is your job to try to diagnose and fix what went wrong. Ready?\nERROR:\n$ERROR"

parsers:

  # Matches line: [1217/201832.950515:ERROR:cache_util.cc(140)] Unable to move cache folder GPUCache to old_GPUCache_000
  - regex: '^\[(\d{4}\/\d{6}\.\d{6}):(?P<LEVEL>\w+):([\w\.\_]+)\(\d+\)\]\s+(?P<MESSAGE>.*)$'

  # Conditions in which the parsed log will trigger a diagnosis
    triggers:
      "LEVEL": "ERROR"

  # Conditions in which the parsed log will be ignored for triggers
    filters:
      "MESSAGE": "401"
      "MESSAGE": "403"

  # Matches line:  2022-01-27 21:37:36.776 0x2eb3     Default       511 photolibraryd: PLModelMigration.m:314   Creating sqlite error indicator file
  - regex: '^(?P<DATE>[^ ]+)\s+(?P<TIME>[^ ]+)\s+[^ ]+(?P<LEVEL>[^ ]+)\s+(?P<MESSAGE>.*)$'

  # When more than one triggers is present, just one trigger is sufficient to trigger a diagnosis
    triggers:
      "LEVEL": "Default"
      "MESSAGE": "(?i)ERROR:"

  # Filters are optional

  # Last parser must always be a generic one that matches any line
  - regex: '^(?P<MESSAGE>.*)$'
  # Both filters and triggers are optional
```

## Examples
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
3. Supports every GPT model version
4. Match multiple log formats in a single log file
5. Match multiple parsers for the same log entry
6. Powerful regex format (Perl/Go flavor)
7. Maximize the amount of log context in the diagnosis

## Work in progress
1. Test dividing log contexts per span, thread, routine or procedure
2. Lightweight docker image
3. Support log filtering

## Future work
1. Structured logging parsing
2. Production readiness (security, auth, monitoring, optimization, more tests...)
3. Release strategy & CI
4. Sentry SDK integration
5. Generate a config.yaml based on real life log examples (use code or GPT to generate regex)
6. Helm chart?
7. Windows / Mac support?
8. Other AI model APIs?
9. Send diagnosis requests to a server for later consumption (agent/server architecture)?

## Testing
`go test ./...`

## Contributing
Feel free to open an issue with your suggestion on how to make this program more useful, portable, efficient and production-ready (and of course BUGS!).

Feel free to open MRs. I'll review them if I see they follow the philosophy of this project. For larger chunks of work or design changes, please open an issue first so that the strategy can be discussed.
