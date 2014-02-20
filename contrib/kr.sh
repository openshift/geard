curl -X PUT "http://localhost:8080/token/__test__/container?u=0&d=1&t=pmorie%2Fsti-html-app&r=0001&i=1" -d '{"ports":[{"external":4343,"internal":8080}]}'
curl "http://localhost:8080/token/__test__/keys?d=1&u=1&i=2" -X PUT -d "{\"keys\":[{\"type\":\"ssh-rsa\",\"value\":\"ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAvzdpZ/3+PUi3SkYQc3j8v5W8+PUNqWe7p3xd9r1y4j60IIuCS4aaVqorVPhwrOCPD5W70aeLM/B3oO3QaBw0FJYfYBWvX3oi+FjccuzSmMoyaYweXCDWxyPi6arBqpsSf3e8YQTEkL7fwOQdaZWtW7QHkiDCfcB/LIUZCiaArm2taIXPvaoz/hhHnqB2s3W/zVP2Jf5OkQHsVOTxYr/Hb+/gV3Zrjy+tE9+z2ivL+2M0iTIoSVsUcz0d4g4XpgM8eG9boq1YGzeEhHe1BeliHmAByD8PwU74tOpdpzDnuKf8E9Gnwhsp2yqwUUkkBUoVcv1LXtimkEyIl0dSeRRcMw==\"}],\"gears\":[{\"id\":\"0001\"}]}"
systemctl restart gear-0001.service

systemctl stop gear-0001.service
systemctl disable gear-0001.service
userdel -r gear-0001