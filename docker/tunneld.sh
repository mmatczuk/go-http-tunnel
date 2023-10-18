#!/bin/sh
CMD="/tunneld --tlsCrt "$CERT_DIR/$PUBLIC_NAME.crt" --tlsKey "$CERT_DIR/$PUBLIC_NAME.key""  
if [[ -z "${CLIENTS}" ]]; then
  echo "no clients were specified"
else
  CMD="${CMD} --clients="$CLIENTS"" 
fi
if [[ "${DEBUG}" == 'true' ]]; then 
  CMD="${CMD} --debug"
  echo "debug on"
fi
if [[ "${DISABLE_HTTPS}" == 'true' ]]; then
  CMD="${CMD} --httpsAddr="" "
  echo "disabled https" 
fi
# run command passed to docker run
echo "$CMD"
$CMD