#!/bin/sh
make uninstall package install || exit 1

set -ux
plakar store rm b2
plakar store add b2 b2://$PLAKAR_B2_BUCKET_NAME key_id=$PLAKAR_B2_KEY_ID app_key=$PLAKAR_B2_APP_KEY
#plakar store add b2 b2://$B2_KEY_ID:$B2_APP_KEY@francoiscolas-plakar
plakar at @b2 create
plakar at @b2 backup ~/Downloads
plakar at @b2 ls
plakar at @b2 restore -to pop