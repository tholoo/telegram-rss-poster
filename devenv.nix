{
  pkgs,
  lib,
  config,
  inputs,
  ...
}:

{
  languages.go = {
    enable = true;
    enableHardeningWorkaround = true;
  };
}
