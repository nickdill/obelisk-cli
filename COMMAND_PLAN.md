

dev - runs dev environment
build - compiles build for prod
run - runs compiled build
deploy - pushes build to servers

install - creates obelisk.yml and .obelisk/*
uninstall - removes .obelisk/* and obelisk.yml


But terminology, install/un usually refer to downloads/modules more than the setup scripts
I would think init sets up scripts
Install would pull modules or somethings... tbd


Maybe modules have: up/down, init/clear, install/uninstall
And servers: ... maybe nothing? its an obelisk app should never remove configs?


So I think a module though
- yes can run with `docker compose up`
- but should also run through obelisk like `obelisk dev`
- to ensure when parent obelisk server runs it that its done the same way
