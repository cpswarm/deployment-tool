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

var refDeployLogs = `install $manager STAGE-START
install $manager compressed to 534 bytes
install $manager sending task
install $agent received announcement
install $agent subscribed to task
install $agent received task
install $agent decompressed archive of 534 bytes
install chmod +x english.sh german.sh EXEC-START
install chmod +x english.sh german.sh EXEC-END
install $agent STAGE-END
run $agent STAGE-START
run ./english.sh EXEC-START
run sleep 5 && ./german.sh EXEC-START
run ./english.sh hi 1
run ./english.sh hi 2
run ./english.sh hi 3
run ./english.sh EXEC-END
run sleep 5 && ./german.sh hallo 1
run sleep 5 && ./german.sh hallo 2
run sleep 5 && ./german.sh hallo 3
run sleep 5 && ./german.sh EXEC-END
run $agent STAGE-END
`
