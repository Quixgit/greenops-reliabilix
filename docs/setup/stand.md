# A demonstration stand on one machine (systemd user units)

For a test deployment that people can open in a browser: API, worker, scheduler and web as `systemctl --user` services,
PostgreSQL and Redis from Docker. Nothing here is for production (it uses development sign-in, `ENV=dev`).

## Layout

```
~/reliabilix-stand/bin/{api,worker,scheduler}   the Go binaries (go build ./cmd/...)
~/reliabilix-stand/web                          the built frontend (a copy of frontend/ with .next and node_modules)
~/.config/reliabilix/{api,worker,scheduler,web}.env   one environment file per service (mode 600). The API and
                                                the worker use different database roles (greenops_api, greenops_worker)
~/.config/systemd/user/reliabilix-*.service     the units in deploy/systemd/
```

## Database

Use a **clean database** for a stand with a scheduler, not one that holds test fixtures: the scheduler fans out over
every organization and connection it finds.

```
createdb greenops_stand
# apply every migration (the "+goose Up" part of backend/migrations/*.sql, or `goose up`)
psql greenops_stand -f deploy/dev/seed.sql        # the empty development organization and operator
psql greenops_stand -f deploy/dev/seed-demo.sql   # optional: a labelled synthetic demo dataset
```

The demo dataset has one AWS connection to a fictitious account. The scheduler unit therefore sets
`SCHEDULER_SKIP=cloudaccounts:sync_all,cloudaccounts:rightsizing_all`, so no synchronization is ever attempted; every
other job (carbon, recommendations, alerts, retention, notifications) runs.

## Run

```
deploy/systemd/install.sh            # copies the units, reloads systemd, enables lingering, starts everything
systemctl --user status reliabilix-api reliabilix-worker reliabilix-scheduler reliabilix-web
journalctl --user -u reliabilix-worker -f
```

`loginctl enable-linger $USER` (done by the script) keeps the services running after logout and starts them at boot.

## Deploy a new version

```
scripts/stand-deploy.sh            # builds the binaries and the frontend with the version in VERSION, restarts the units
STAND_DB_CONTAINER=<postgres container> STAND_DB=greenops_stand scripts/stand-deploy.sh migrate 0037 0038   # first: the numbers of the migrations added since the last deploy
```

The stand serves the frontend straight from the checkout, so a `next build` changes the files under the running web
process. Always restart the web unit right after a build (the script does); a server that was started before the build
answers pages it had not rendered yet with `ChunkLoadError`.
