## Test Setup

We create 10 mutating webhooks that intecepts configMap creation, then run
`batchNumber` threads to create configMaps, each creating `batchSize` configMaps
in serial. We measure the run time of the client.

## Results

Latency in seconds for Direct connection between kube-apiserver and cluster:

| #batches\batch size | 1   | 10   | 100  | 1000 |
|---------------------|-----|------|------|------|
| 1                   |     |      | 2.0  | 21.1 |
| 10                  |     | 1.1  | 9.7  |      |
| 100                 | 1.3 | 10.3 |      |      |

Latency in seconds for connection going through network proxy running in GRPC
mode:

| #batches\batch size | 1   | 10   | 100  | 1000 |
|---------------------|-----|------|------|------|
| 1                   |     |      | 24.9 | 243  |
| 10                  |     | 12.0 | 89.0 |      |
| 100                 | 16.8| 101  |      |      |

Latency in seconds for connection going through network proxy running in
HTTP-Connect mode:

| #batches\batch size | 1   | 10   | 100  | 1000 |
|---------------------|-----|------|------|------|
| 1                   |     |      | 31.2 | 240  |
| 10                  |     | 17.9 | 152  |      |
| 100                 | 19.7| 86   |      |      |

Latency for ssh connection: TODO

## Use this test suite

TODO
