
### Инструкция по созданию своих листов RULES

На текущий момент DNS-сервер поддерживает два вида поиска:

* DOMAIN-SUFFIX - wildcard домена, т.е любой домен заканчивающийся на указанное
* DOMAIN-MATCH - домен содержит указанный текст в любом месте
* DOMAIN - только домен полное соответствие

Каждое правило занимает одну строку в формате

[ВидПоиска],[Строка],[НазваниеДействия]

Правила обрабатываются по порядку и первое подходящее правило останавливает обработку. Рекомендуется самые популярные списки (инстаграмм, ютуб и тд) указывать в начале списка

Название [Действия] определяется в конфигурации DNS сервера. Если действие не определено в конфигурации (config.yaml) обработка пропускается

[Действие]
```sh
actions:
 PROXY: # Название действия PROXY
 #...
 PROXY2:  # Название действия PROXY2
 #...
```

### Примеры

```sh
DOMAIN-SUFFIX,example.com,PROXY
# -> example.com
# -> test.example.com
# -> test1.example.com ... etc
DOMAIN-SUFFIX,example1.com,PROXY
# -> example.com
# -> test.example1.com
# -> test1.example1.com ... etc
DOMAIN-SUFFIX,youtube.com,PROXY
# -> youtube.com
# -> accounts.youtube.com ... etc
DOMAIN-MATCH,you,PROXY
# -> youtube.com 
# -> accounts.youtube.ae
# -> you.com
# -> yourself.com ... etc
```

### Дополнительные опции

* RULE-SET - список правил который будет загружен при загрузке Rules. Формат такой же как в этом документе. Действие в файле не учитывается и применяется указанное в RULE-SET. Загрузка по http/https.

```sh
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Instagram/Instagram.list,PROXY
# Каждому правило загруженному по этому адресу будет присвоено действие указанное после запятой 
```

### Пример реализации

Пример списка можно найти в ./example/rules.txt

