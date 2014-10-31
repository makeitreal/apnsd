# apnsd

apns + redis = apnsd

## usage

```
go get github.com/makeitreal/apnsd/cmd/apnsd
apnsd -apnsCer="path/to/cerfile" -apnsKey="path/to/keyfile"
```

### enqueue msg

Send LPUSH command to redis-server with key that apnsd watch and message encoded as messagepack.
Default redis key is ```APNSD:MSG_QUEUE```.

#### msg spec

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

### failed msg

When APNS server receive wrong msg, return error response include error status and identifier (detail is [here](https://developer.apple.com/library/ios/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/Chapters/CommunicatingWIthAPS.html)). Apnsd send lpush command to redis server with key and msg include error status encoded as messagepack when received. Default redis key is ```APNSD:FAILED_MSG_QUEUE```.

#### failed msg spec

```
{
    token:      binary
    payload:    string encoded as json
    expire:     uint32
    priority:   uint8
    identifier: uint32
    status:     uint8
}
```

## todo

* feedback service
* write document
* graceful shutdown

## feature
