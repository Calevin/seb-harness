{
  description = "Entorno de desarrollo para el motor Go de seb-harness";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forEachSystem = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      devShells = forEachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            # Paquetes necesarios en tu entorno de desarrollo
            packages = with pkgs; [
              go
              sqlite # Útil para consultar la base de datos de auditoría localmente
            ];

            # Variables de entorno por defecto (opcional, fuerza CGO apagado por seguridad)
            CGO_ENABLED = "0";

            shellHook = ''
              echo "Entorno de desarrollo para seb-harness activado."
              echo "Herramientas disponibles: go $(go version | awk '{print $3}'), sqlite3."
              echo ""
              echo "Para inicializar y compilar el motor, ejecuta:"
              echo "  mkdir -p .seb/cmd/seb .seb/bin"
              echo "  cd .seb"
              echo "  go mod init seb-harness/seb"
              echo "  go get modernc.org/sqlite@latest"
              echo "  CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o bin/seb ./cmd/seb/main.go"
              echo "  cd .."
            '';
          };
        }
      );
    };

}
