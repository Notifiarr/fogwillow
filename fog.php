<?php
$host = "192.168.1.193";
$socket = socket_create(AF_INET, SOCK_DGRAM, SOL_UDP);
$start = microtime(true);

foreach (glob("/tmp/stuff/1060/*.log") as $filename) {
    $input = explode("\n", file_get_contents($filename));
    $outputfile = $filename . ".udp";

    $truncate = "2\ntruncate=true\nfilepath=".$outputfile."\n";
    socket_sendto($socket, $truncate, strlen($truncate), 0, $host, 9000);

    //echo "$filename size " . filesize($filename) . "\n";
    foreach ($input as $lineIndex => $line) {
        if ($lineIndex+1 != count($input)) {
            $line .= "\n";
        }
        socket_clear_error($socket);
        socket_put_contents($socket, $outputfile, $line, $host);
        $error = socket_last_error($socket);
        $errorText = socket_strerror($error);
        $sockError = socket_get_option($socket, SOL_SOCKET, SO_ERROR);
        echo 'Line: '. ($lineIndex + 1) .', socket_last_error: ' . $error . ', socket_strerror: ' . $errorText . ', sock error:' . $sockError . "<br>\n";
        usleep(1);
    }
}

echo 'Duration: ' . number_format((microtime(true) - $start), 5) . '<br>';


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
?>
