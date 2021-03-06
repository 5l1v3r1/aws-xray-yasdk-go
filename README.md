# aws-xray-yasdk-go
Yet Another [AWS X-Ray](https://aws.amazon.com/xray/) SDK for Go

The Yet Another AWS X-Ray SDK for Go is compatible with Go 1.13 and above.

## TODO

- implement ECS plugin
- implement EKS plugin
- implement beanstalk plugin

## Configuration

### Environment Values

- `AWS_XRAY_DAEMON_ADDRESS`: Set the host and port of the X-Ray daemon listener. By default, the SDK uses `127.0.0.1:2000` for both trace data (UDP) and sampling (TCP).
- `AWS_XRAY_CONTEXT_MISSING`: `LOG_ERROR` or `RUNTIME_ERROR`. The default value is `LOG_ERROR`.
- `AWS_XRAY_TRACING_NAME`: Set a service name that the SDK uses for segments.
- `AWS_XRAY_DEBUG_MODE`: Set to `TRUE` to configure the SDK to output logs to the console
- `AWS_XRAY_LOG_LEVEL`: Set a log level for the SDK built in logger. it should be `debug`, `info`, `warn`, `error` or `silent`. This value is ignored if `AWS_XRAY_DEBUG_MODE` is set.
- `AWS_XRAY_SDK_ENABLED`: Disabling the SDK. It is parsed by [`strconv.ParseBool`](https://golang.org/pkg/strconv/#ParseBool) that accepts `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, `False`. The default value is `true`.

### Code

These configure overwrites the environment configure.

```go
// configure the daemon address and the context missing strategy.
xray.Configure(&xray.Config{
  DaemonAddress:          "127.0.0.1:2000",
  ContextMissingStrategy: &ctxmissing.RuntimeErrorStrategy{},
})

// configure the default logger.
xraylog.SetLogger(NewDefaultLogger(os.Stderr, xraylog.LogLevelDebug))
```

## Quick Start

### Start a custom segment/subsegment

```go
func DoSomethingWithSegment(ctx context.Context) error
  ctx, seg := xray.BeginSegment(ctx, "service-name")
  defer seg.Close()

  ctx, sub := xray.BeginSubsegment(ctx, "subsegment-name")
  defer sub.Close()

  err := doSomething(ctx)
  if sub.AddError(err) { // AddError returns the result of err != nil
    return err
  }
  return nil
}
```

### HTTP Server

```go
func main() {
  namer := xrayhttp.FixedTracingNamer("myApp")
  h := xrayhttp.Handler(namer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, "Hello World!")
  }))
  http.ListenAndServe(":8000", h)
}
```

### HTTP Client

```go
func getExample(ctx context.Context) ([]byte, error) {
  req, err := http.NewRequest(http.MethodGet, "http://example.com")
  if err != nil {
    return nil, err
  }
  req = req.WithContext(ctx)

  client = xrayhttp.Client(nil)
  resp, err := client.Do(req)
  if err != nil {
      return nil, err
  }
  defer resp.Body.Close()
  return ioutil.ReadAll(resp.Body)
}
```

### AWS SDK

```go
sess := session.Must(session.NewSession())
dynamo := dynamodb.New(sess)
xrayaws.Client(dynamo.Client)
dynamo.ListTablesWithContext(ctx, &dynamodb.ListTablesInput{})
```

### SQL

```go
func main() {
  db, err := xraysql.Open("postgres", "postgres://user:password@host:port/db")
  row, err := db.QueryRowContext(ctx, "SELECT 1")
}
```

## See Also

- [AWS X-Ray](https://aws.amazon.com/xray/)
- Official [AWS X-Ray SDK for Go](https://github.com/aws/aws-xray-sdk-go)
