# Fog Willow

## Log Buffer

This app listens on a UDP port. Parses packets with a specific format. Buffers the packet payloads and writes them to disk in larger batches.

Allows us to efficiently transport logs out of the PHP app and
directly to the system with the hard disk where we store log files.
We use this because NFS was too slow, and we can trade a bit of reliability for faster log exfiltration.


## Packet Format

The first line of the packet is an integer that signals how many settings to expect.
A minimum of 1 settings is required: `filepath=/path`
Other recognized [settings](https://github.com/Notifiarr/fogwillow/blob/main/pkg/fog/set.go#L8) are:
`flush=true`, `truncate=true`, `delete=true`, `password=p4ssw0rd`

### Example Packets

```text
1
filepath=/path/to/file.log
[INFO] this is a log line
```

```text
2
filepath=/path/to/file.log
password=option4lPassw0rd
[INFO] this is a log line
```

Create a packet with `echo` and `netcat`:
```bash
echo -e "1\nfilepath=/tmp/filename.txt\nfile content goes here\nline 2 in the file" | nc -uw0 127.0.0.1 9000
```

## Metrics

Has a Prometheus exporter built in with juicy [metrics](https://github.com/Notifiarr/fogwillow/blob/main/pkg/metrics/metrics.go)!<br/>
[![grafana](https://github.com/Notifiarr/fogwillow/wiki/images/grafana-thumb.png "grafana images")](https://github.com/Notifiarr/fogwillow/wiki/images/grafana.png)

## PHP Client

Very simple procedure we use for testing.

```php
$socket = socket_create(AF_INET, SOCK_DGRAM, SOL_UDP);

function socket_put_contents($socket, $outputfile, $line, $host, $length = 8000)
{
    if (strlen($line) > $length) {
        foreach (str_split($line, $length) as $piece) {
            $loggerPayload = "1\nfilepath=".$outputfile."\n".$piece;
            usleep(1);
            socket_sendto($socket, $loggerPayload, strlen($loggerPayload), 0, $host, 9000);
        }
    } else {
        $loggerPayload = "1\nfilepath=" . $outputfile . "\n" . $line;
        socket_sendto($socket, $loggerPayload, strlen($loggerPayload), 0, $host, 9000);
    }
}
```

# Usage

We run this from a Docker container directly on a Synology NAS. Using this image:<br/>
`ghcr.io/Notifiarr/fogwillow:main`

Mount `/config` and give it a `/config/fog.conf` file that looks like the [example](https://github.com/Notifiarr/fogwillow/blob/main/fog.conf). You also want to mount a place to store files, and set that to `output_path` in the config file.
