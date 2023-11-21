# API说明

## 设备上线/离线监控

当未授权手机插入时，会发送以下消息
```
time="2023-11-21T17:16:09+08:00" level=info msg="adb-monitor: {Serial:6d0a931b OldState:StateDisconnected NewState:StateOffline}"
time="2023-11-21T17:16:09+08:00" level=info msg="adb-monitor: {Serial:6d0a931b OldState:StateOffline NewState:StateAuthorizing}"
time="2023-11-21T17:16:09+08:00" level=info msg="adb-monitor: {Serial:6d0a931b OldState:StateAuthorizing NewState:StateUnauthorized}"
```

此时如点击授权，则会继续发送以下消息
```
time="2023-11-21T17:16:30+08:00" level=info msg="adb-monitor: {Serial:6d0a931b OldState:StateUnauthorized NewState:StateOffline}"
time="2023-11-21T17:16:30+08:00" level=info msg="adb-monitor: {Serial:6d0a931b OldState:StateOffline NewState:StateOnline}"
```

当接入一台已授权的手机插入时，会顺序发送以下消息
```
"monitor: {Serial:PQY0220A15002880 OldState:StateDisconnected NewState:StateOffline}"
"monitor: {Serial:PQY0220A15002880 OldState:StateOffline NewState:StateAuthorizing}"
"monitor: {Serial:PQY0220A15002880 OldState:StateAuthorizing NewState:StateOffline}"
"monitor: {Serial:PQY0220A15002880 OldState:StateOffline NewState:StateOnline}"
```

设备离线

```
time="2023-11-21T17:16:58+08:00" level=info msg="adb-monitor: {Serial:6d0a931b OldState:StateOnline NewState:StateOffline}"
time="2023-11-21T17:16:58+08:00" level=info msg="adb-monitor: {Serial:6d0a931b OldState:StateOffline NewState:StateDisconnected}"
```

总结：只需监听以下连个状态变化，其他变化可以忽略
1. OldState:StateOnline NewState:StateOffline，则触发离线回调
2. OldState:StateOffline NewState:StateOnline，则触发上线回调