Это DNS сервер который на базе конфигураций ShadowRocket поможет обходить точечно блокировки подменяя адрес на локальный (fakeip). 

* Поддерживает ipv4/ipv6
* Поддерживает все виды записей включая RRSIG (dnssec)
* Правила имеют формат like ShadowRocket/ClashX
* Кэширует запросы

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
    # !!! Скрипты являются примером. В каждом скрипте нужно поменять действующий интерфейс (сейчас там указан ens3)
    ```
    Пример запуска 
    ```sh
    ./unicornDNS -config example/config.yaml -rules example/rules.txt
    ```
    !!! S

 Правила настраиваются по списку который -rules config/rules.txt. Пример не сложный. В текущей реализации он распознает только:

* DOMAIN-SUFFIX – любой домен *.google.com или 
* DOMAIN-KEYWORD,google,PROXY - любое google.am, mail.google.com подходит под этот фильтр. Все что не подошло под правила – пропускается без каких-либо fakeip как есть.
* RULE-SET - скачать список (в примере так же есть все)

## 2. Дополнительно

Я встроил HTTP сервер для возможности удаленного сброса или обновления rules. 
*  "/clearcache" Сброс кэша DNS с удалением правил
*  "/reload" Перезагрузка правил (RULES)
*  "/ips" Просмотр аренды fakeip адресов
*  "/rules" Список  действующих правил

При каких-либо проблемах я предусмотрел скрипты для отчистки. Сам сервер по завершению отчищает, но мало ли :) 
* sh/iptables/ipv4_reset_all_rules.sh
* sh/iptables/ipv6_reset_all_rules.sh

### [Пример реализации](doc/ROS7.md)

### [Пример простой конфигурации](example/config.yaml)

### [Пример конфигурации с описанием](example/config_2.yaml)

### [Пример правил](example/rules.txt)

### [Документация по правилам](doc/rules_ru.md)