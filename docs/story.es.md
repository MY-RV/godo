# Por qué existe GoDo

El repo debería traer una lista compartida de lo que se puede correr. Mismos nombres, mismo significado, para quien llegue al árbol.

En Go, Java, C#, Rust, Python y compañía a menudo no hay un archivo delgado que diga “estas son las tareas con nombre de este repo”. Quedan pedazos de README, carpetas `scripts/`, botones del IDE, YAML de CI, comandos largos que pegas cuando te acuerdas. No está mal. Son mapas a medias. Sirven si ya conoces el lugar. Cuestan cuando alguien nuevo necesita un solo sitio: aquí `check`, aquí `seed`, aquí lo que corre antes del shell.

Si trabajas con JavaScript en el día a día, ya conoces una buena versión de eso. `package.json` tiene `scripts`. Ese campo es un catálogo: tareas con nombre en el repo, fáciles de lanzar. Muchos equipos no necesitan más. Sí — en ese ecosistema eso llegó temprano. El referente claro, si ya lo usas.

Incluso ahí suele moverse otra pieza: no tanto los nombres, sino con qué binario abres el proyecto — `npm`, `pnpm`, `yarn` o `bun`. Si los mezclas, pelean los lockfiles. Por eso existen `packageManager` y Corepack: dejar fijo el runner para que un clone no adivine. Muchos equipos ya lo usan. El drift es costumbre, no destino.

Los atajos personales intentaron llenar el hueco. En el día a día con **Naoki** en [Rosvelt](https://rosvelt.com/), los entrypoints largos duelen: el lenguaje no ayuda a tratarlos como scripts de repo, y el trabajo cruza lenguajes. `nkd` era un helper en `.zshrc` para eso. La máquina murió y se fue con ella — local, listo. Después, un router privado de [MY-RV](https://github.com/MY-RV) (`myrv`) podía despachar formas largas desde una laptop. Útil en una máquina. No un catálogo que todo el mundo versiona en el repo. Si te pasó algo parecido, ya conoces el hueco. Otro alias privado no lo tapa. Algo que el repo pueda cargar, sí.

Por eso **GoDo**. Un `godo.yaml`. Lo listas, lo previsualizas, lo corres:

```bash
godo --ls
godo --preview check
godo check
```

Los comandos pueden ser lo que sea: `go test`, `dotnet`, `cargo`, una línea de shell. El punto son los nombres compartidos.

También es una apuesta pública. Ya había [goxdi](https://github.com/MY-RV/goxdi) afuera; la idea era no dejarlo solo — sacar algo serio que otros puedan usar, no solo drafts en la laptop.

Práctico, sin sermón: si un repo JS ya tiene `scripts` sólidos y el package manager fijado, tal vez no necesitas otro catálogo. Cuando el árbol cruza lenguajes, o los entrypoints reales son formas largas que nadie quiere reescribir, un archivo chico compartido ayuda. Mejor preview antes de confiar.

[Getting started](./getting-started.md) · [English](./story.md)
