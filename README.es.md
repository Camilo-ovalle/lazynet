# lazynet

Una TUI y CLI "lazy" para equipos Cisco **Catalyst** (IOS / IOS-XE), **Nexus** (NX-OS) y **ASA**,
inspirada en lazygit y lazydocker.

*[Read in English](README.md)*

- **Dos formas de trabajar:** `lazynet` abre una interfaz interactiva (teclado + mouse), y los subcomandos sirven para tareas rápidas y scripts.
- **Solo lectura por defecto:** todo usa comandos `show`, y el código lo garantiza. El único cambio posible es el cambio de VLAN de acceso, que siempre muestra los comandos exactos y pide confirmación.
- **Detección automática** de Catalyst, Nexus o ASA.
- **Sin dependencias:** un único binario estático, con SSH, git (para los backups) y keyring integrados.

## Instalación

```sh
curl -fsSL https://raw.githubusercontent.com/Camilo-ovalle/Lazy_Net/main/scripts/install.sh | sh
```

Con **apt** (Debian / Ubuntu):

```sh
curl -fsSL https://camilo-ovalle.github.io/Lazy_Net/lazynet.gpg | sudo tee /usr/share/keyrings/lazynet.gpg >/dev/null
echo "deb [signed-by=/usr/share/keyrings/lazynet.gpg] https://camilo-ovalle.github.io/Lazy_Net stable main" | sudo tee /etc/apt/sources.list.d/lazynet.list
sudo apt update && sudo apt install lazynet
```

También puedes descargar el `.deb`, `.rpm` o tarball desde [releases](https://github.com/Camilo-ovalle/Lazy_Net/releases) (`sudo apt install ./lazynet_*.deb`)
o usar `go install github.com/Camilo-ovalle/Lazy_Net/cmd/lazynet@latest`.

## Probarlo sin red

```sh
lazynet --demo                       # TUI con Catalyst, Nexus y ASA emulados
lazynet --demo vlan set core-sw1 -p 1/0/7 -v 20
```

El modo demo levanta equipos falsos en localhost. No toca tu red, tu keyring ni tus archivos.

## Uso rápido

```sh
lazynet inventory init                                   # crea ~/.config/lazynet/inventory.yml
lazynet info core-sw1                                    # serial, MAC, IP, versión de OS, uptime
lazynet iface core-sw1 -p 1/0/25                         # estado, VLAN, velocidad, errores
lazynet logs core-sw1 --from "2026-11-01 08:00" --to "2026-11-01 18:00"
lazynet backup --group core && lazynet backup diff core-sw1
lazynet vlan set core-sw1 --port Gi1/0/1 --vlan 110
```

En la TUI: `enter` conecta, `1-4` cambia de pestaña (Info, Interfaz, Logs, Backups), `p` consulta una interfaz, `v` cambia una VLAN, `b` toma un backup,
`f` `/` `s` filtran los logs por fecha, texto y severidad, `a` conecta a una IP fuera del inventario y `?` muestra la ayuda. El mouse funciona para clics y scroll.

## Seguridad

- **Cambio de VLAN:**
  1. Valida el puerto, rechaza trunks y verifica que la VLAN exista, todo con `show`.
  2. Muestra los comandos exactos y espera una `y` explícita.
  3. Verifica el resultado leyendo el puerto de nuevo.
  4. Pregunta aparte si hacer `write memory`.
  5. Registra todo en `~/.local/share/lazynet/audit.log`.
- **Credenciales:** las contraseñas se piden, **nunca se guardan en disco**, y se recuerdan durante **5 minutos** (como `sudo`). En Linux viven en el keyring del kernel con expiración automática.
- **Host keys:** se confía en la llave en la primera conexión (TOFU), guardada en `~/.config/lazynet/known_hosts`. Una llave que cambia es un error grave. La llave se verifica *antes* de pedir la contraseña.
- **Privilegios:** se requiere un login con privilegio 15 (prompt `#`).

Más detalles (formato del inventario, `config.yml`, desarrollo) en el [README en inglés](README.md).
