Это DNS сервер который на базе конфигураций ShadowRocket поможет обходить точечно блокировки подменяя адрес на локальный. Поддерживает ipv4/ipv6

# Сборка и настройка проекта UnicornDNS

## 1. Установка

### Шаги для сборки
1. Сборка сервера:
    ```sh
    git clone https://github.com/unicorn-style/unicornDNS/
    cd unicornDNS
    go build
    ```
    В папке sh/iptables находятся sh скрипты. Им нужно дать право на исполнение
    ```sh
    sudo chmod +x sh/iptables
    ```
    Пример запуска 
    ```sh
    ./unicornDNS -config example/config.yaml -rules example/rules.txt
    ```

## 2. Пример реализации RouterOS7 <-> Wireguard <-> VPS (с установленным unicornDNS)

Пример подойдет для людей которые уже туннель создали и немного понимают как работает маршрутизация.

### Настройки туннеля на VPS unicornDNS (далее Gate)
```sh
inteface name wg0
[Interface]
PrivateKey = ******
Address = 10.10.13.2/30, 2001:db8::2/64
ListenPort = 13232

[Peer]
PublicKey = ******
Endpoint = address_ROS7:13231
AllowedIPs = 10.56.0.0/24, 10.10.13.0/30, 2001:db8::/64
```
* 10.56.0.0/24 - Это моя внутренняя сеть за !ROS
* 10.10.13.0/30 – Это сеть которая создается внутри туннеля
* 2001:db8::/64 - Это сеть ipv6 которая так же создается внутри туннеля

### Настройки туннеля на стороне RouterOS 6/7 (далее ROS)
```sh
inteface name wgate
[Interface]
PrivateKey = ******
Address = 10.10.13.1/30, 2001:db8::1/64
ListenPort = 13232

[Peer]
PublicKey = ******
Endpoint = address_VPS:13231
AllowedIPs = 0.0.0.0/0, ::/0
```

На ROS7 нам надо добавить сеть которая будет использоваться для маршрутизации. Для примера я буду использовать: 

* 10.199.0.0/24 для ipv4

* 2001:db9:aaaa:aaaa::/117 для ipv6

Если вам не нужно ipv6, вы можете опустить это. 
```sh
ip route add 10.199.0.0/24 %wgate
ip6 route add 2001:db9:aaaa:aaaa::/117 %wgate
```
За счет маршрутов мы говорим ROS где искать конкретные адреса, а именно отправлять все запросы на 10.199.0.0 / 2001:db9:aaaa:aaaa:: в туннель 10.10.13.2 / 2001:db8::2

## 2. Переходим к настройке unicornDNS (Gate)

Заходим и копируем конфигурацию в отдельную папку для теста. И переходим к редактированию.
```sh
cd unicornDNS
cp example config
sudo chmod +x sh -R
sudo chmod +x unicornDNS
nano config/config.yaml
```

Тут задаются подсети которые будут использованы для генерации адресов. В этом примере секция должна выглядеть так:

```yaml
networks:
  ipv4:
    cidr: "10.199.0.0/24"
  ipv6:
    cidr: "2001:db9:aaaa:aaaa::/117" 
```
PS: Для большего понимания я оставил дополнительную справку в самих конфиг файлах: example/config.yaml, example/config_2.yaml


bind-address секцию не трогаем на время тестирования работы. DNS сервер создается на порту 8053

```yaml
  bind-address: "0.0.0.0:8053"
```
Вам нужно зайти в каждый файл sh и изменить ens3 на интерфейс основной который у вас на VPS (интерфейс с интернетом). После этого можно начинать тестировать

```sh
ls sh/iptables
```
После того как все это сделали все готово к, 
Запускаем тестирование

```sh
./unicornDNS -config config/config.yaml -rules config/rules.txt
```

После запуска сервера вам достаточно для теста сделать 

```sh
dig @10.10.13.2 -p 8053 youtube.com

Вывод должен быть примерно таким: 

;; QUESTION SECTION:
;youtube.com.			IN	A

;; ANSWER SECTION:
youtube.com.		99	IN	A	10.199.0.17
```

Далее меняем и запускаем.
```yaml
  bind-address: "0.0.0.0:53"
```

Остается на ROS сделать полностью передачу всех запросов или "точечно" с помощью FWD DNS на сервер 10.10.13.2. Правила настраиваются по списку который -rules config/rules.txt. Пример не сложный. В текущей реализации он распознает только:

* DOMAIN-SUFFIX – любой домен *.google.com или 
* DOMAIN-KEYWORD,google,PROXY - любое google.am, mail.google.com подходит под этот фильтр. Все что не подошло под правила – пропускается без каких-либо fakeip как есть.
* RULE-SET - скачать список (в примере так же есть все)

## 3. Дополнительно

Я встроил HTTP сервер для возможности удаленного сброса или обновления rules. 
* - "/clearcache" Сброс кэша DNS с удалением правил
* - "/reload" Перезагрузка конфигруации правил

При каких-либо проблемах я предусмотрел скрипты для отчистки. Сам сервер по завершению отчищает, но мало ли :) 
* sh/iptables/ipv4_reset_all_rules.sh
* sh/iptables/ipv6_reset_all_rules.sh



