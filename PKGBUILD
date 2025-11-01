# Maintainer: Trance233<huaji2475785724@163.com>

pkgname=trance-cli
pkgver=1.0.0
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
    "${pkgname}::git+https://github.com/popjdh/trance-cli.git#tag=v1.0.0-bugfix"
)
sha256sums=(
    'SKIP'
)

build() {
	cd "$srcdir"/"$pkgname"
    CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath -o "$pkgname" main.go
    ./"$pkgname" completion bash > "$pkgname".bash
}

package() {
	cd "$srcdir"
    install -Dm755 "$pkgname/$pkgname" "$pkgdir/usr/bin/$pkgname"
    install -Dm644 "$pkgname/$pkgname.bash" "$pkgdir"/etc/bash_completion.d/"$pkgname".bash
}