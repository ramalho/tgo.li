# check — confere os redirecionamentos

```
usage: check [-base URL] [-c N] [-per-host N] [-timeout DURATION] FILE
  -base string
        base URL to check short paths against (default: derived from the
        .htaccess file name, e.g. TGO.LI.htaccess -> https://tgo.li/)
  -c int
        number of concurrent requests (default 20)
  -per-host int
        maximum concurrent requests to any one destination host (default 4)
  -timeout duration
        per-request timeout (default 15s)
```

`check` lê as diretivas `RedirectTemp` do arquivo, visita cada URL curta como
um navegador faria — seguindo os redirecionamentos até o fim — e relata as que
não terminam numa resposta 2xx.

Cada problema é reportado numa linha com três campos separados por tab:
o que houve, o caminho curto e a URL longa. Exemplo:

```sh
$ check TGO.LI.htaccess
404 	2b	https://exemplo.com/pagina-que-sumiu
T/O 	3k	https://servidor.lento.example/
??? 	4m	https://dominio-que-nao-resolve.example/x
```

`404` e os demais números são o status HTTP da resposta final; `T/O` é um
tempo esgotado; `???` é qualquer outra falha em conseguir uma resposta — a
conexão caiu, o DNS não resolveu, os redirecionamentos não acabavam mais.
Se não houver problemas, nada é exibido.

As linhas aparecem na ordem em que as respostas chegam, não na ordem do arquivo:
numa verificação de muitas URLs isso mostra o progresso, em vez de ficar mudo
até a última requisição resolver.

## A quem pertence o domínio

Sem `-base`, o domínio vem do nome do arquivo: `TGO.LI.htaccess` cuida de
tgo.li, então os caminhos são conferidos sob `https://tgo.li/`. Use `-base`
para conferir os mesmos caminhos em outro lugar — um ambiente de teste, ou o
arquivo antes de publicar:

```sh
$ check -base http://localhost:8080/ TGO.LI.htaccess
```

## Educação com os servidores alheios

Duas medidas evitam que o próprio `check` crie os erros que iria relatar:

`-per-host` limita quantas requisições simultâneas caem no mesmo host de
destino (4, por padrão), para que um punhado de caminhos curtos apontando
todos para o mesmo site não o afogue — o que voltaria como erro de limite de
taxa e se pareceria com um link morto.

`check` também se apresenta com o `User-Agent` de um navegador. Muitos sites
— Wikipedia e O'Reilly entre eles — respondem 403 ao `User-Agent` padrão do
Go, e esses falsos positivos afogariam os links de fato mortos.
