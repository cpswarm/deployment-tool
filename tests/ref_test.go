package tests

var refSums = map[string]string{
	"english.sh": "6762bb660bf0c5d547732db03d5baf1a",
	"german.sh":  "5b9d5d897f68cf6c8de3f6b03cd1792f",
}

var refDeploy = []byte(`
source:
  zip: UEsDBAoAAAAAADpdBE/2qqquPwAAAD8AAAAKAAAAZW5nbGlzaC5zaCMhL2Jpbi9iYXNoCgpmb3IgaSBpbiB7MS4uM30KZG8KICAgZWNobyAiaGkgJGkiCiAgIHNsZWVwIDEKZG9uZVBLAwQKAAAAAAA6XQRPe3Rl3UIAAABCAAAACQAAAGdlcm1hbi5zaCMhL2Jpbi9iYXNoCgpmb3IgaSBpbiB7MS4uM30KZG8KICAgZWNobyAiaGFsbG8gJGkiCiAgIHNsZWVwIDEKZG9uZVBLAQIUAAoAAAAAADpdBE/2qqquPwAAAD8AAAAKAAAAAAAAAAAAAAAAAAAAAABlbmdsaXNoLnNoUEsBAhQACgAAAAAAOl0ET3t0Zd1CAAAAQgAAAAkAAAAAAAAAAAAAAAAAZwAAAGdlcm1hbi5zaFBLBQYAAAAAAgACAG8AAADQAAAAAAA=

deploy:
  install:
    commands:
      - chmod +x english.sh german.sh

  run:
    commands:
      - ./english.sh
      - sleep 5 && ./german.sh

  target:
    ids:
    tags:
      - swarm

debug: true
`)

var refDeployLogs = map[string]string{
	//////////////////
	"install $manager": `
STAGE-START
compressed to 534 bytes
sending task`,
	////////////////
	"install $agent": `
received announcement
subscribed to task
received task
decompressed archive of 534 bytes
STAGE-END`,
	///////////////////////////////////////
	"install chmod +x english.sh german.sh": `
EXEC-START
EXEC-END`,
	////////////
	"run $agent": `
STAGE-START
STAGE-END`,
	//////////////////
	"run ./english.sh": `
EXEC-START
hi 1
hi 2
hi 3
EXEC-END`,
	////////////////////////////
	"run sleep 5 && ./german.sh": `
EXEC-START
hallo 1
hallo 2
hallo 3
EXEC-END`,
}
