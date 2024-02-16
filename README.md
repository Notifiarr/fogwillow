# Log buffer

This app listens on a UDP port. Parses packets with a specific signature. Buffers the packet payloads and writes them to disk.

Allows us to efficiently transport logs out of the PHP app and directly to the system with the hard disk where we store log files.

See [config file example](https://github.com/Notifiarr/fogwillow/blob/main/fog.conf).