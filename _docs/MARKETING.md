Deploy and manage apps everywhere

CLI to push things to prod in one command
Dashboard breaking down each servers heath
If obelisk, get in depth stats/metrics, break down docker services, admin panel, etc
Status pages
Redundant servers (deploy east/west, regional, anything)
Load balancers

Give claude code the ability to build and deploy its own services
Give it a sandbox and limited compute and have it remove and clean space



So like would a user do
```
obelisk new prod-obelisk
obelisk new stag-obelisk
obelisk new local-obelisk
```
it creates /prod-obelisk, /stag-obelisk, /local-obelisk
and goes in and edits the conf for each and deploy each?
(at this point in theory should be able to go in and `obelisk dev` or `obelisk run`

Or would it be a single `obelisk new obelisk-app` and then each environment has an obelisk.env something


Think about even higher level, as a dev how do I organize and manage a growing and faster growing number of projects.
If I have many different git repos and cant keep track, Megalisk perhaps the tool to manage.
But especially with deploymentts, having a primary Obelisk that you configure the module list in git

Maybe the trick is, users when they `obelisk deploy` might not use git, might not tag versions, might not commit everything etc
So we just take current push and tag/version ourselves 

But wed have potentially a dev obelisk with all modules on it just to track manage
Then megalisk/obelisk admin can display the full list
and each individual obelisk can configure its own module list based on the needs/deployed hardware
So dev obelisk is maybe just branded obelisk app, like ps-obelisk and dev config has all modules
If you want some deployments to only host some modules, give deployment config override 
(the build scripts will check those first and pull only specified images, nginx generation and docker generation already expected done)


The true obelisk is the packaged solid rock at the end (aka the react app is compiled, the rails app image is built, nginx conf already includes everything in the modules config list
(Also tangent, but should be able to test the nginx/docker-compose generation is deterministic. Delete the nginx and call cmd again it should create, try again it shouldnt change, remove and app and build it should remove

"carving release" like stone monolith, while building for deploy

Dev env can skip build and just run from source `obelisk dev` generates script changes and runs docker from generated compose
Prod/staging should run from builds, `obelisk run` from `obelisk build` (generates script changess for all modules)

tbd builds where too

`obelisk deploy` takes the build or maybe runs build? depends... but essentially pushes conf changes to ... i guess thise goes to a default server or maybe all servers else you probably need to target a specific server with `obelisk deploy prod` or `obelisk deploy stage` or feature/test env `obelist deploy DEV-420` 



