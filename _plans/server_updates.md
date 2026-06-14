Given I have created an obelisk server via `obelisk init`
When the Obelisk server template gets updated
Then I should be able to pull those updates with `obelisk server update` 
AND I can get specific template versions with `obelisk server update 1.2.3`

We need to think about how we will handle conflicts. If a user has an obelisk.yml defined and the new version adds ned values we can't just override the old we need to migrate. So likely we'll need a better migration tool? Or just manual updates? Suggestions are welcome.

I am deploying the prototype of this server to EC2 and need to be able to pull the latest updates without having to reinstall fresh each time
