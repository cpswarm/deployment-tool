# Documentation

## Contents
1. [Components](#components)
2. [Target](#target)
3. [Task Description](#task-description)
4. [Task](#task)
5. [Target Registry](#target-registry)

## Components
The Deployment Tool consists of two components: 
* **Deployment Manager**: a centralized web service exposing APIs for various deployment-related operations.
* **Deployment Agent**: a web service running on individual devices, performing deployment-related commands receivced 
 from the Deployment Manager.

## Target
Target is a device that hosts a running instance of the Deployment Agent. Each Target has a unique ID and a set of tags 
 (e.g. device type, group) which identify the device. The ID is set as part of Deployment Agent configuration. The tags
 can be configured similarly to the ID or remotely via the Deployment Manager API.

## Task Description
Task Description is a set of instructions and configurations which describe an intended deployment process. This process
 includes typical deployment steps such as assembly, transfer, installation, testing, and activation. In addition, the
 Task Description provides information about target devices and logging requirements.
 
The Task Description is submitted to the Deployment Manager via the RESTFul API of the service. The API supports both
 YAML and JSON representations during task submission. Currently, all API responses are in JSON but the support for YAML
 responses is foreseen with HTTP content type negotiations (i.e. using Accept header).
 
In the example below, a package is assembled locally in a directory named `package` and transferred to devices with tags 
 `groupA` and `turtlebot`. Once the transfer is complete, the package is installed and then the predefined validation 
 tests are executed. The package is activated as soon as the test succeeds and after system startup. The package can also be 
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

In response, the API will provide the submitted description along with the size of compressed package and list of
 matching targets:

```json
  {
    "Stages": {
      "Assemble": [
        "make",
        "make clean"
      ],
      "Transfer": [
        "package"
      ],
      "Install": [
        "chmod +x package/monitor",
        "chmod +x package/rescue"
      ],
      "Test": [
        "sh test-script.sh"
      ],
      "Activate": [
        "on-success",
        "on-startup",
        "manually"
      ]
    },
    "Target": {
      "Tags": [
        "groupA",
        "turtlebot"
      ]
    },
    "Log": {
      "Interval": "3s",
      "Verbosity": "DEBUG"
    },
    "DeploymentInfo": {
      "TaskID": "784f439c4034-8347-11e8-85b6-a0da33d3",
      "TransferSize": 6548204,
      "MatchingTargets": [
        "975ab24c-f2dc-4c9e-952c-d546b246d179"
      ]
    }
  }
```

## Task
Task is an instantiation of the Task Description consisting all necessary information for performing the deployment on 
 a target. This usually includes the compressed package data, installation and test steps, and activation triggers.
 
## Target Registry
The Deployment Manager maintains the list of Targets. The list is populated either by registering Targets manually or 
 via a discovery mechanism. The Deployment Manager provides a RESTFul API for managing the Targets.
 
Below is the Target Registry index after deployment of the above task. The example registry consists of one target with 
 three tags. Since the `groupA` tag from the Task Description matches this target, the target has received the tasks and 
 provided the status, output logs, time taken to perform the tasks, as well its task history. 

```json
[
  {
    "ID": "975ab24c-f2dc-4c9e-952c-d546b246d179",
    "Tags": [
      "mypc",
      "macbook",
      "groupA"
    ],
    "Tasks": {
      "LatestBatchResponse": {
        "ResponseType": "SUCCESS",
        "Responses": [
          {
            "Command": "chmod +x package/monitor",
            "Stdout": "exit status 0",
            "Stderr": "",
            "LineNum": 1,
            "TimeElapsed": 0.008383173
          },
          {
            "Command": "chmod +x package/rescue",
            "Stdout": "exit status 0",
            "Stderr": "",
            "LineNum": 1,
            "TimeElapsed": 0.016520635
          }
        ],
        "TimeElapsed": 0.016520635,
        "TaskID": "784f439c4034-8347-11e8-85b6-a0da33d3",
        "TargetID": "975ab24c-f2dc-4c9e-952c-d546b246d179"
      },
      "History": [
        "784f439c4034-8347-11e8-85b6-0668a1c0",
        "784f439c4034-8347-11e8-85b6-a0da33d3"
      ]
    }
  }
]
```

