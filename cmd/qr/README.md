# qr — gera o QR code

```
usage: qr [PATH|URL]
```

`qr` grava um PNG com o QR code no diretório atual e escreve na saída padrão a
URL codificada. O argumento pode ser um caminho curto ou uma URL inteira; sem
argumento, `qr` lê a URL da entrada padrão:

```sh
$ qr xy7                    # caminho curto: ganha o prefixo https://tgo.li/
https://tgo.li/xy7          # gravado em xy7.png
$ qr gopl.io                # tem ponto: é URL, codificada como veio
gopl.io                     # gravado em gopl.io.png
$ qr https://gopl.io/ch1
https://gopl.io/ch1         # gravado em gopl.io-ch1.png
```

Uma URL curta do próprio tgo.li é nomeada só pelo caminho: `https://tgo.li/22`
vira `22.png`.

Nos demais casos o nome do PNG vem da URL inteira: o esquema cai fora e cada
sequência de caracteres que um nome de arquivo não aceita — as barras,
principalmente — vira um único hífen. Maiúsculas são preservadas, porque o caminho de uma URL
diferencia maiúsculas de minúsculas e juntá-las daria o mesmo arquivo a duas
páginas diferentes. Um PNG existente é sobrescrito: a URL que ele codifica não
muda.

## Ligando os dois

Sem argumento, `qr` lê a primeira linha da entrada padrão — e como ele descarta
o comentário no fim da linha, a saída do `tgo` serve como está:

```sh
$ tgo https://gopl.io/ | qr
https://tgo.li/23           # gravado em 23.png
```

Com argumento dá na mesma, porque `qr` junta os argumentos que recebe antes de
ler a linha:

```sh
$ qr $(tgo https://gopl.io/)
https://tgo.li/23

$ tgo https://gopl.io/ | xargs qr
https://tgo.li/23
```

Só um `#` isolado — com espaço dos dois lados — abre comentário. O `#` colado
na URL continua sendo fragmento:

```sh
$ qr https://example.com/a#top
https://example.com/a#top   # gravado em example.com-a-top.png
```
