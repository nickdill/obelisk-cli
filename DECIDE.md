should obelisk be a 
or


obelisk up
obelisk build
obelisk deploy (stage/prod)
down

to initialize an app as obelisk module
ie to ship a public "obelisk module" just need an app that spins up on `obelisk up` which does like .obelisk/run.sh

`obelisk` 
So when you have an obelisked app, how to add it to the deployed obelisk

`obelisk servers` list active servers
- highlight ones that have this module
`obelisk remove` cli asks what server to remove this from
`obelisk add` cli asks what server to add to
- complexity on this one is how to persist, if you write to obelisk.yml it can get lost on redeploys/updates/restarts - so adding to a deployed obelisk... via git? ie push commits to add it then prod obelisk does docker pull/run?
- obelisk could auto pr with added service? annoying
- ideally I go to the admin module, see the list of services/modules, just add/remove and the instance persists that forever, like the .env file?
`obelisk deploy` cli asks what server to 

So need like a server save/git maybe? If I do `obelisk save` or similar to post to obelisk cloud. Can `obelisk load` to get a server.
So really these are just templates. Feed it a template file,...

Summary
What Im trying to solve is, I have a deployment running a docker obelisk. I built a new app and want it deployed.
`obelisk add` lets you add

well so we need the project repo pushed to git so we can pull it
then need to give repo to docker compose, overall kind of tricky - not sure it'd work for private repos?...



So `obelisk init` to turn current project into an obelisk module
It creates config files
Then `obelisk up` to try and run the obelisk app, test your config
Then `obelisk build` to create a build with compiled assets
Then `obelisk deploy` to push the latest build to all servers
(If first time theres also `obelisk add` to add this module to an obelisk you have deployed)

`obelisk list` displays all deployed obelisks (prod/stagings/N feature envs)
`mega list` should display all servers...
`obelisk` refers to a single server, it hosts obelisk modules

ie consider you have a deployed instance and run `obelisk` and havent connected yet, whats that process look like?

Can literally have a servers.yml or a deployments section in the obelisk.yml and `obelisk deploy` just pushes to those obelisks
ie reinvent devops by working application up instead of having to click to each server and manage
