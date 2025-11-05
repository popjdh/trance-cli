# Maintainer: Trance233<huaji2475785724@163.com>

pkgname=trance-cli
_pkgname=trance
pkgver=1.0.2
pkgrel=1
pkgdesc="A script snippet collection"
arch=('x86_64')
url=""
license=('custom')
depends=()
optdepends=(
    'openssh: For "trance ssh" support'
    'libjxl: For "trance img cjxl" support'
)
makedepends=(go)
options=(!debug)
source=(
    "${pkgname}::git+https://github.com/popjdh/trance-cli.git#tag=v${pkgver}"
)
sha256sums=(
    'SKIP'
)

build() {
	cd "$srcdir"/"$pkgname"
    CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath -o "$_pkgname" main.go
    ./"$_pkgname" completion bash > "$_pkgname".bash
}

package() {
	cd "$srcdir"
    install -Dm755 "$pkgname/$_pkgname" "$pkgdir/usr/bin/$_pkgname"
    install -Dm644 "$pkgname/$_pkgname.bash" "$pkgdir"/etc/bash_completion.d/"$_pkgname".bash
}