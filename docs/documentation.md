# Documentation

## Contents
2. [Target](#target)
1. [Task Description](#task-description)
3. [Task](#task)

## Target


## Task Description
Task Description is a set of instructions and configurations which describe an intended deployment process. This process
 includes typical deployment steps such as assembly, transfer, installation, testing, and activation. In addition, the
 Task Description provides information about target devices and logging requirements.
 
The Task Description is submitted to the Deployment Manager via the RESTFul API of the service. The API supports both
 YAML and JSON representations during task submission. Currently, all API responses are in JSON but the support for YAML
 responses is foreseen with HTTP content type negotiations (i.e. using Accept header).
 
In the example below, a package is assembled locally in a directory named `package` and transferred to devices with tags 
 `groupA` and `turtlebot`. Once the transfer is complete, the package is installed and then the predefined validation 
 tests are executed. The package is activated as soon as the test succeeds, after system startup. The package can also be 
 deactivated/activated manually using the API. Throughout the whole process, log messages up to `DEBUG` verbosity are 
 collected every `3s`. The status of the deployment and log messages can be retrieved using the API. 
```yaml
stages:
  assemble: # commands run by manager (e.g. packaging, cross-compilation)
    - make
    - make clean
  transfer: # list of files and directories copied to targets
    - package
  install: # commands run by agent
    - chmod +x package/monitor
    - chmod +x package/rescue
  test:
    - sh test-script.sh
  activate:
    - on-success # when tests pass
    - on-startup
    - manually # signal from AL or Command & Monitoring Tool (CMT)

target:
  tags:
    - groupA
    - turtlebot

log:
  interval: 3s
  verbosity: DEBUG
```

## Task