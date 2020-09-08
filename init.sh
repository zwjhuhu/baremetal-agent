# put into /opt/vyatta/etc/config/scripts/vyatta-postconfig-bootup.script
BASEDIR=/boot/grub
SOURCERTAR=$BASEDIR/zvr.tar.gz
SBIN_DIR=/opt/vyatta/sbin

if [ ! -f $SOURCERTAR ]; then
   echo "cannot find the tar file"
   $SBIN_DIR/zvrboot >/home/vyos/zvr/zvrboot.out 2>&1 < /dev/null &
   exit 0
fi

tmpdir=$(mktemp -d)
ZVR=$tmpdir/zvr
ZVRBOOT=$tmpdir/zvrboot
ZVRSCRIPT=$tmpdir/virtualrouteragent
HAPROXY=$tmpdir/haproxy
GOBETWEEN=$tmpdir/gobetween
HEALTHCHECK=$tmpdir/healthcheck.sh
VERSION=`date +%Y%m%d`
ZVR_VERSION=$tmpdir/version

cp -f /etc/security/limits.conf $tmpdir/limits.conf
grep -w "vyos" $tmpdir/limits.conf  | grep soft || echo "vyos soft nofile 1000000" >> $tmpdir/limits.conf
grep -w "vyos" $tmpdir/limits.conf  | grep hard || echo "vyos hard nofile 1000000" >> $tmpdir/limits.conf
cp -f $tmpdir/limits.conf /etc/security/limits.conf

tar xzf $SOURCERTAR -C $tmpdir
echo "$VERSION" > /etc/version
cp -f $ZVR $SBIN_DIR/zvr
cp -f $ZVRBOOT $SBIN_DIR/zvrboot
cp -f $ZVRSCRIPT /etc/init.d/virtualrouteragent
cp -f $HAPROXY $SBIN_DIR/haproxy
cp -f $GOBETWEEN $SBIN_DIR/gobetween
mkdir -p /home/vyos/zvr/
cp -f $ZVR_VERSION /home/vyos/zvr/version
cp -f $HEALTHCHECK /usr/share/healthcheck.sh

chmod +x $SBIN_DIR/zvrboot
chmod +x $SBIN_DIR/zvr
chmod +x /etc/init.d/virtualrouteragent
chmod +x $SBIN_DIR/haproxy
chmod +x $SBIN_DIR/gobetween
chmod +x /usr/share/healthcheck.sh

chown vyos:users /home/vyos/zvr
chown vyos:users $SBIN_DIR/zvr
chown vyos:users $SBIN_DIR/haproxy
chown vyos:users $SBIN_DIR/gobetween
chown vyos:users /usr/share/healthcheck.sh
$SBIN_DIR/zvrboot >/home/vyos/zvr/zvrboot.out 2>&1 < /dev/null &

rm -f $SOURCERTAR
rm -rf $tmpdir
