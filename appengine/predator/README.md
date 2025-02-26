# Predator
Predator is a culprit finder for Chrome crashes identified or reported on
[ClusterFuzz](https://cluster-fuzz.appspot.com), Cracas and UMA Sampling Profiler. Key features include:
* Suggest suspected CLs for a Chrome crash which has a regression range.
* For crashes on Cracas, we use metadata to infer the regression range.
* Suggest components for bug filing for all Chrome crashes/regressions.

# Integration with other systems
* Predator is integrated with Cracas to publicly-reported Chrome crashes. Analysis results are reported back to Cracas and surface on Cracas UI for stability sheriffs.

# Code structure

* [analysis](analysis): Core of analysis, this directory is not dependent on any
  infra or google app engine libraries.
  The predator.py is the entry point which we create changelist_classifier to
  find suspected cls, component_classifier to find suspected components and
  project_classifier to find suspected project.
* [app](app): The App Engine modules to provide heuristic analysis of Predator.
* [scripts](scripts): Utility scripts to run locally on developer's workstation.
* [first\_party](first_party): Libraries from sibling infra AppEngine app that Predator depends on, specifically including a few libs from [Findit](../findit).
* [third\_party](third_party): Third-party libraries that Predator depends on.

# How to checkout code and make change?
Run the below sample commands:

**mkdir predator-dev**

**cd predator-dev**

**fetch infra**

**cd infra/appengine/predator**

**git checkout -b dev**


# How to deploy to appengine?
**cd infra/appengine/predator/app** and run the below commands on your cloudtop:

**make deploy_staging** to deploy changes on Staging

**make deploy_prod** to deploy changes on Production

# Monitoring

(1) Internal dashboards to monitor all the results:

* Clusterfuzz: https://predator-for-me.appspot.com/clusterfuzz/dashboard

* Cracas: https://predator-for-me.appspot.com/cracas/dashboard

* UMA Sampling Profiler: https://predator-for-me.appspot.com/uma-sampling-profiler/dashboard

(2) Monitor metrics using vicerory dashboards:

* go/predator-metrics

# Testing and Debug

Run **cd infra/appengine/predator/app** and verify:

* Unit Test: Run **make pytest** and ensure all unit test cases passed

* Debug on Staging: Run **make deploy_staging** to deploy changes on Staging.
  Please notice that ABF discards requests from Predator Staging,
  so no tickets will be created for Predator Staging.
  So it is very common to verify changes on Staging.
  However, we should be very careful about other use cases as the Staging traffic
  mirrors the Production traffic.


