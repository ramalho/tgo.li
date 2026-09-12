# tgo — cria o caminho curto

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

## URLs equivalentes

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
