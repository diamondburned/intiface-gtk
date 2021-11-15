{
	lib,
	src ? ./..,
	vendorSha256 ? lib.fakeSha256,
	pkgs,
	internalPkgs ? import ./pkgs.nix {}, # only for overriding
}:

internalPkgs.buildGoModule {
	inherit src vendorSha256;

	pname = "intiface-gtk";
	version = "0.0.1-tip";

	buildInputs = with internalPkgs; [
		gtk4
		glib
		graphene
		gdk-pixbuf
		gobjectIntrospection
	];

	nativeBuildInputs = with pkgs; [ pkgconfig ];
}
