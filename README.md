# tgo.li

Encurtador de URLs feito em casa.

Não há servidor: os redirecionamentos ficam num arquivo `.htaccess` publicado
na raiz do site tgo.li, e dois comandos cuidam dele.

## Instalação

```sh
go install github.com/ramalho/tgo.li/cmd/tgo@latest
go install github.com/ramalho/tgo.li/cmd/qr@latest
```

Ou, dentro do repositório, `go build ./cmd/tgo ./cmd/qr`.

## tgo — cria o caminho curto

```
usage: tgo [-f FILE] URL
  -f string
        path to the .htaccess file (default "TGO.LI.htaccess")
```

`tgo` procura a URL no arquivo. Se ela ainda não estiver lá, acrescenta uma
diretiva `RedirectTemp` com o próximo caminho livre. Em ambos os casos escreve
uma linha na saída padrão com a URL curta e um comentário dizendo o que
aconteceu:

```sh
$ tgo https://go.dev/doc/effective_go
https://tgo.li/22  # new
$ tgo https://go.dev/doc/effective_go
https://tgo.li/22  # existing
```

O arquivo fica assim:

```
# TGO.LI redirects — managed by the tgo command
RedirectTemp /22	https://go.dev/doc/effective_go
```

Os caminhos são gerados em ordem (`22`, `23`, ..., `zz`, `222`, ...) com o
alfabeto `23456789abcdefghjkmnpqrstvwxyz` — sem `0`, `1`, `i`, `l` e `u`, que
se confundem ao ditar ou copiar uma URL de um material impresso.

### URLs equivalentes

Antes de comparar, `tgo` normaliza a URL pelas três equivalências da RFC 3986
que valem por definição: o esquema e o host não diferenciam maiúsculas de
minúsculas, e um caminho ausente equivale a `/`. Duas grafias assim recebem o
mesmo caminho curto.

O resto fica como veio. URLs que diferem só por uma barra final, pelo prefixo
`www.` ou por `http`/`https` **podem** ser a mesma página, mas nada garante
isso — cada uma ganha seu próprio caminho curto, e `tgo` avisa na saída de
erro:

```sh
$ tgo https://docs.python.org/3/howto/
https://tgo.li/24  # new
note: /23 already redirects to https://docs.python.org/3/howto
	(differs only by a trailing slash)
```

Como o aviso sai em stderr, a saída padrão continua sendo uma única linha,
pronta para entrar no `qr` por um pipe.

## qr — gera o QR code

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

Uma URL curta do próprio tgo.li é nomeada só pelo caminho — `https://tgo.li/22`
vira `22.png`, como era com o antigo `tgo -q`.

Nos demais casos o nome do PNG vem da URL inteira: o esquema cai fora e cada
sequência de caracteres que um nome de arquivo não aceita — as barras,
principalmente — vira um único hífen. Maiúsculas são preservadas, porque o caminho de uma URL
diferencia maiúsculas de minúsculas e juntá-las daria o mesmo arquivo a duas
páginas diferentes. Um PNG existente é sobrescrito: a URL que ele codifica não
muda.

### Ligando os dois

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

## Publicação

Copie `TGO.LI.htaccess` para a raiz do site tgo.li com o nome `.htaccess`.
