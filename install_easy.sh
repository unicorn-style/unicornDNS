#!/bin/sh

# Начальные переменные
IFACE=""
URL="https://raw.githubusercontent.com/unicorn-style/unicornDNS/main"
INSTALLTYPE="ipv4"
INSTALLPATH="/usr/local/bin"
RULES=""
DEFAULT_IPV4_CIDR="10.199.0.0/16"
DEFAULT_IPV6_CIDR="2001:db9:aaaa:aaaa::/117"
HTTP_BINDADDRESS="127.0.0.1:8081"
DNS_BINDADDRESS="0.0.0.0:53"
DNS_FORWARD="1.1.1.1:53"
FIREWALL_TYPE="iptables"  # по умолчанию iptables
TNAME_NAT=""

# Получение архитектуры системы
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="x86_64" ;;
  aarch64) ARCH="arm64" ;;
  arm*) ARCH="arm" ;;
  *) echo "Not support: $ARCH"; exit 1 ;;
esac

# DOWNLOAD_URL="https://github.com/unicorn-style/unicornDNS/releases/latest/download/unicornDNS-$ARCH.tar.gz"
DOWNLOAD_URL="https://raw.githubusercontent.com/unicorn-style/unicornDNS/main/build/unicornDNS.tar.gz"
# Получение интерфейсов
INTERFACES=$(ip -o link show | awk -F': ' '{print $2}')
i=1
for iface in $INTERFACES; do
  echo "$i. $iface"
  i=$((i + 1))
done

# Выбор интерфейса
echo "Enter WAN-interface number:"
read IFACE_INDEX

i=1
for iface in $INTERFACES; do
  if [ $i -eq $IFACE_INDEX ]; then
    IFACE=$iface
    break
  fi
  i=$((i + 1))
done

# Выбор типа установки
echo "Select installation type:"
echo "1. ipv4"
echo "2. ipv4ipv6"
echo "3. ipv6"
read INSTALLTYPE_INDEX

case $INSTALLTYPE_INDEX in
  1)
    INSTALLTYPE="ipv4"
    DEFAULT_IPV6_CIDR=""
    ;;
  2)
    INSTALLTYPE="ipv4ipv6"
    ;;
  3)
    INSTALLTYPE="ipv6"
    DEFAULT_IPV4_CIDR=""
    ;;
  *)
    echo "Invalid choice. Defaulting to ipv4."
    INSTALLTYPE="ipv4"
    ;;
esac

# Запрос на выбор cidr для ipv4
if [ "$INSTALLTYPE" = "ipv4" ] || [ "$INSTALLTYPE" = "ipv4ipv6" ]; then
  echo "Enter IPv4 CIDR (default $DEFAULT_IPV4_CIDR):"
  read IPV4_CIDR
  IPV4_CIDR=${IPV4_CIDR:-$DEFAULT_IPV4_CIDR}
fi

# Запрос на выбор cidr для ipv6
if [ "$INSTALLTYPE" = "ipv6" ] || [ "$INSTALLTYPE" = "ipv4ipv6" ]; then
  echo "Enter IPv6 CIDR (default $DEFAULT_IPV6_CIDR):"
  read IPV6_CIDR
  IPV6_CIDR=${IPV6_CIDR:-$DEFAULT_IPV6_CIDR}
fi

# Запрос на выбор bind-address и dns-forward
echo "Enter bind address and port for DNS-server(default $DNS_BINDADDRESS):"
read BIND_ADDRESS
BIND_ADDRESS=${BIND_ADDRESS:-$DNS_BINDADDRESS}

# Проверка занятости порта
BIND_PORT=$(echo $BIND_ADDRESS | awk -F':' '{print $2}')
if netstat -tuln | grep -q ":$BIND_PORT "; then
  echo "Port $BIND_PORT is already in use. Please choose another port. End."
  exit 1
fi

echo "Enter default DNS forward address (default $DNS_FORWARD):"
read DNS_FORWARD_INPUT
DNS_FORWARD=${DNS_FORWARD_INPUT:-$DNS_FORWARD}

echo "Enter default HTTP address ip:port for controll this server. This  (по умолчанию $HTTP_BINDADDRESS):"
read HTTP_BINDADDRESS_INPUT
HTTP_BINDADDRESS=${HTTP_BINDADDRESS_INPUT:-$HTTP_BINDADDRESS}

# Проверка наличия nftables и выбор механизма firewall
if command -v nft > /dev/null 2>&1; then
  echo "Select firewall mechanism:"
  echo "1. iptables"
  echo "2. nftables"
  read FIREWALL_INDEX

  case $FIREWALL_INDEX in
    1)
      FIREWALL_TYPE="iptables"
      ;;
    2)
      FIREWALL_TYPE="nftables"
      # Предложение создать таблицы для маршрутизации
      echo "Do you want to create nftables chains? [Y/N]:"
      read CREATE_TABLES
      if [ "$CREATE_TABLES" = "Y" ] || [ "$CREATE_TABLES" = "y" ]; then
        # Уточнение имени цепочки/фильтра
        echo "Enter NAT chain name (default: nat):"
        read TNAME_NAT_INPUT
        TNAME_NAT=${TNAME_NAT_INPUT:-"nat"}

        # echo "Enter filter name (default: dnsFILTER):"
        #  read TNAME_FILTER_INPUT
        # TNAME_FILTER=${TNAME_FILTER_INPUT:-"dnsFILTER"}
      fi
      ;;
    *)
      echo "Invalid choice. Defaulting to iptables."
      FIREWALL_TYPE="iptables"
      ;;
  esac
else
  echo "nftables not found. Use default iptables."
  FIREWALL_TYPE="iptables"
fi

# Выбор пути установки
echo "Would you like to change the installation directory (default: $INSTALLPATH)? [Y/N]:"
read CHANGE_INSTALLPATH

if [ "$CHANGE_INSTALLPATH" = "Y" ] || [ "$CHANGE_INSTALLPATH" = "y" ]; then
  echo "Enter new installation directory:"
  read INSTALLPATH
fi

# Подтверждение выбора
echo "Your settings:"
echo "- WAN Interface: $IFACE"
echo "- Installation networks: $INSTALLTYPE"
echo "– IPv4 CIDR: ${IPV4_CIDR:-Not Set}"
echo "- IPv6 CIDR: ${IPV6_CIDR:-Not Set}"
echo "- Bind address: $BIND_ADDRESS"
echo "- DNS forward address: $DNS_FORWARD"
echo "- HTTP bind address: $HTTP_BINDADDRESS"
echo "- Firewall: $FIREWALL_TYPE"
if [ "$FIREWALL_TYPE" = "nftables" ]; then
  echo "NAT chain name: $TNAME_NAT"
  echo "Filter name: $TNAME_FILTER"
fi
echo "Installation directory: $INSTALLPATH"
echo "System architecture: $ARCH"
echo "Do you confirm the installation? [Y/N]:"
read CONFIRM

if [ "$CONFIRM" != "Y" ] && [ "$CONFIRM" != "y" ]; then
  echo "Installation canceled."
  exit 0
fi

# Выполнение установки
INSTALL_DIR="$INSTALLPATH/unicornDNS"
CONFIG_FILE="$INSTALL_DIR/config.yaml"

# Создание директории
mkdir -p "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR/sh"

# Генерация конфигурационного файла

export BIND_ADDRESS="$BIND_ADDRESS"
export DNS_FORWARD="$DNS_FORWARD"
export HTTP_BINDADDRESS="$HTTP_BINDADDRESS"
export IFACE="$IFACE"
export TNAME_NAT="$TNAME_NAT"
export DEFAULT_IPV4_CIDR="$DEFAULT_IPV4_CIDR"
export DEFAULT_IPV6_CIDR="$DEFAULT_IPV6_CIDR"
export INSTALLTYPE="$INSTALLTYPE"

curl -o "tmp_config_$FIREWALL_TYPE.yaml" "$URL/src/tmp_config_$FIREWALL_TYPE.yaml"
envsubst < tmp_config_$FIREWALL_TYPE.yaml > "$INSTALL_DIR/config.yaml"
rm tmp_config_$FIREWALL_TYPE.yaml

unset BIND_ADDRESS
unset DNS_FORWARD
unset HTTP_BINDADDRESS
unset IFACE
unset TNAME_NAT
unset DEFAULT_IPV4_CIDR
unset DEFAULT_IPV6_CIDR
unset INSTALLTYPE

# Скачивание и распаковка сборки в зависимости от архитектуры
curl -o "$INSTALL_DIR/unicornDNS.tar.gz" "$DOWNLOAD_URL"
mkdir $INSTALL_DIR/unicornDNS
tar -xzf "$INSTALL_DIR/unicornDNS.tar.gz" -C "$INSTALL_DIR/unicornDNS"
rm "$INSTALL_DIR/unicornDNS.tar.gz"

# Скачивание скриптов
curl -o "$INSTALL_DIR/sh/add.sh" "$URL/sh/$FIREWALL_TYPE/add.sh"
curl -o "$INSTALL_DIR/sh/delete.sh" "$URL/sh/$FIREWALL_TYPE/delete.sh"
curl -o "$INSTALL_DIR/sh/onReset.sh" "$URL/sh/$FIREWALL_TYPE/onReset.sh"
curl -o "$INSTALL_DIR/rules.list" "https://raw.githubusercontent.com/unicorn-style/unicorndns_rkn/main/rules.list"

echo "Do you want to install UnicornDNS as a system service? [Y/N]:"
read INSTALL_SERVICE

if [ "$INSTALL_SERVICE" = "Y" ] || [ "$INSTALL_SERVICE" = "y" ]; then
  # Detect system type and install service
  if command -v systemctl > /dev/null 2>&1; then
    # Using systemd
    echo "Systemd detected. Installing service."

    SERVICE_FILE="/etc/systemd/system/unicorndns.service"

    # Create service file
    cat <<EOF > "$SERVICE_FILE"
[Unit]
Description=UnicornDNS Service
After=network.target

[Service]
ExecStart=$INSTALL_DIR/unicornDNS -rules $INSTALL_DIR/rules.txt
WorkingDirectory=$INSTALL_DIR
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF
    # Install service in systemd
    systemctl daemon-reload
    systemctl enable unicorndns
    systemctl start unicorndns
    echo "UnicornDNS successfully installed and started as a service."
  fi
fi

echo "UnicornDNS successfully installed at $INSTALLPATH"
echo " – Config file: $INSTALLPATH/config.yaml"
echo " – Rules file: $INSTALLPATH/rules.list"
echo "\n\nUse these commands to controll"
echo "\"http://$HTTP_BINDADDRESS/reset\" - reset firewall and cache"
echo "\"http://$HTTP_BINDADDRESS/clearcache\" - flush DNS-cache server"
echo "\"http://$HTTP_BINDADDRESS/reload\" - reload rule list"