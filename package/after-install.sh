#!/bin/sh

# This file is used by deb, rpm and BSD packages.
# FPM adds this as the after-install script.

mkdir -p '/var/log/fogwillow'

if [ -x "/bin/systemctl" ]; then
  # Reload and restart - this starts the application as user nobody.
  /bin/systemctl daemon-reload
  /bin/systemctl enable fogwillow
  /bin/systemctl restart fogwillow
fi
