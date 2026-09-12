# tgo.li

Encurtador de URLs feito em casa.

Não há servidor: os redirecionamentos ficam num arquivo `.htaccess` publicado
na raiz do site tgo.li, e três comandos cuidam dele.

## Os três comandos

- **[`tgo`](cmd/tgo/README.md)** — cria o caminho curto. Recebe uma URL longa,
  procura por ela no `.htaccess` e, se ainda não estiver lá, acrescenta uma
  diretiva `RedirectTemp` com o próximo caminho livre. Escreve a URL curta na
  saída padrão.
- **[`qr`](cmd/qr/README.md)** — gera o QR code. Recebe um caminho curto ou uma
  URL inteira (ou lê a URL da entrada padrão, para receber a saída do `tgo`
  por um pipe) e grava um PNG com o QR code no diretório atual.
- **[`check`](cmd/check/README.md)** — confere os redirecionamentos. Visita
  todas as URLs curtas definidas no `.htaccess` e lista as que não devolvem
  `OK 200`.

## Instalação

```sh
go install github.com/ramalho/tgo.li/cmd/tgo@latest
go install github.com/ramalho/tgo.li/cmd/qr@latest
go install github.com/ramalho/tgo.li/cmd/check@latest
```

Ou, dentro do repositório, `go build ./cmd/tgo ./cmd/qr ./cmd/check`.

## Publicação

Copie `TGO.LI.htaccess` para a raiz do site tgo.li com o nome `.htaccess`.
