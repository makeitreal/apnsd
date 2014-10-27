# apnsd

apns + redis = apnsd

## usage

```
go get github.com/makeitreal/apnsd/cmd/apnsd
apnsd -apnsCer="path/to/cerfile" -apnsKey="path/to/keyfile"
```

### enqueue

Send LPUSH command to redis-server with key that apnsd watch and message encoded as messagepack.
Default redis key is ```APNSD:MSG_QUEUE```.

### encode spec

```
{
    token:      binary
    payload:    string encoded as json
    expire:     uint32
    priority:   uint8
    identifier: uint32
}
```

If identifier is 0, apnsd set identifier with incrmented number. 
Expire and priority spec is [here](https://developer.apple.com/library/ios/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/Chapters/CommunicatingWIthAPS.html).

Payload size should be less than or equal to 256 bytes. apnsd trim payload body ( aps.alert or aps.alert.body ) automatically.

## todo

* feedback service
* write document
* graceful shutdown

## feature
