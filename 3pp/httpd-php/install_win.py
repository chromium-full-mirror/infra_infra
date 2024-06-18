#!/usr/bin/env python3
# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import os
import tarfile
import shutil
import subprocess
import sys
import zipfile

apr_version = "1.7.2"
apr_iconv_version = "1.2.2"
apr_util_version = "1.5.4"
gawk_version = "3.1.6-1"
httpd_version = "2.4.55"
openssl_version = "1.1.1j"
pcre_version = "8.45"
php_version = "8.2.5"
zlib_version = "1.2.13"


def nmake_windows_builder(init_directory):
  os.chdir("./httpd")

  if os.environ.get("_3PP_PLATFORM") == "windows-amd64":
    os.environ["CL"] = "/D_WIN32_WINNT#0x0600 /D \"WIN64\" /D \"_WIN64\""
    os.environ["_LINK_"] = "/MANIFEST /MACHINE:x64"
  elif os.environ.get("_3PP_PLATFORM") == "windows-arm64":
    os.environ["CL"] = "/D_WIN32_WINNT=0x0A600 /D \"ARM64\" /D \"_ARM64_\""
    os.environ["_LINK_"] = "/MANIFEST /MACHINE:arm64"
  res = subprocess.run(["nmake", "-f", "Makefile.win", "_buildr"])
  print("exit process for nmake is %d" % res.returncode)
  if (res.returncode != 0):
    print("nmake -f Makefile.win _buildr failed")
    os.chdir("./srclib/apr/tools")

    # TODO(nihardamar): right now this is a bit messy where we allow the
    # makefile to fail, then make fixes on the fly. A good refactor for
    # future would be to make fixes to makefile in order to either
    # 1. use the correct compilation settings within makefile
    # 2. remove the step with gen_test_char building and just build it ourselves
    #    before running makefile

    # need to recompile gen_test_char executable as a 64 bit executable
    # as the host machine is x64.
    # The issue is that gen_test_char must be compiled for the host platform,
    # rather than the target platform. Target platform is arm64.
    res = subprocess.check_call([
        "gcc", "-Wall", "-O2", "-DCROSS_COMPILE", "gen_test_char.c", "-s", "-o", "gen_test_char"])
    print("gcc res = ", res)

    os.chdir("..")
    os.remove("./LibR/gen_test_char.exe")
    os.rename("./tools/gen_test_char.exe", "./LibR/gen_test_char.exe")

    # arm64 will need retry after gen_test_char.exe failures so it doesn't
    # this way it doesn't override the newly built x64 executable.
    os.chdir(init_directory)
    os.chdir("./httpd")

    # this failure is expected, apr makefile will build a arm64 executable,
    # we will build our own x64 executable after then re-run
    print("retry - gcc failure for arm64")
    os.remove("./srclib/apr/include/apr_escape_test_char.h")
    os.system(
        ".\\srclib\\apr\\LibR\\gen_test_char.exe > .\\srclib\\apr\\include\\apr_escape_test_char.h"
    )

    res = subprocess.run(
        ["nmake", "-f", "Makefile.win", "_buildr", "CPU=ARM64"])

    if res.returncode != 0:
      # similar to above now the httpd gen_test_char will fail due to same issue,
      # running makefile again now will restart without overriding our newly made
      # x64 executable for the host platform.
      print("second gcc failure for gen_test_char")

      os.chdir("./server")
      os.remove("./gen_test_char.exe")
      res = os.system(
          "gcc -Wall -O2 -DCROSS_COMPILE gen_test_char.c -s -o gen_test_char")
      print("gcc 2 res  = ", res)
      os.system("gen_test_char.exe > test_char.h")

      os.chdir(init_directory)
      os.chdir("./httpd")
      res = subprocess.run(
          ["nmake", "-f", "Makefile.win", "_buildr", "CPU=ARM64"])

      if res.returncode != 0:
        print("gcc failed")
        return sys.exit(res.returncode)
      else:
        print("completed compiling windows")
        os.chdir(init_directory)
        return 0
    else:
      os.chdir(init_directory)
      print("completed compiling windows")
      return sys.exit(res.returncode)

  os.chdir(init_directory)


def build_openssl(init_directory):
  os.chdir("./httpd/srclib/openssl")

  res = 1

  if os.environ.get("_3PP_PLATFORM") == "windows-amd64":
    res = subprocess.run(["perl.bat", "Configure", "no-comp", "VC-WIN64A"])
  elif os.environ.get("_3PP_PLATFORM") == "windows-arm64":
    res = subprocess.run(["perl.bat", "Configure", "no-comp", "VC-WIN64-ARM"])
  else:
    res = subprocess.run(["perl.bat", "Configure", "no-comp", "VC-WIN32"])
  if (res.returncode != 0):
    print("open ssl perl failed")
    return sys.exit(res.returncode)

  res = subprocess.run(["nmake", "-f", "makefile"])

  if (res.returncode != 0):
    print("open ssl nmake failed")
    return sys.exit(res.returncode)

  os.chdir(init_directory)


def enable_openssl_apr():
  with open("./httpd/srclib/apr-util/include/apu.hw", "r") as f:
    lines = f.readlines()
  with open("./httpd/srclib/apr-util/include/apu.hw", "w") as f:
    for line in lines:
      if "APU_HAVE_CRYPTO" in line:
        print("apu replaced crypto 0 with 1")
        f.write(line.replace("0", "1"))
      else:
        f.write(line)


def remove_base(file_path, base_num):
  # Base addresses are hard coded for x64/x86 machines, these old addresses won't work
  # for arm64. Instead we'll just remove it and let the compiler figure out where to
  # have the address.
  with open(file_path, "r") as f:
    lines = f.readlines()
  with open(file_path, "w") as f:
    for line in lines:
      if f"/base:\"{base_num}\"" in line:
        line = line.replace(f"/base:\"{base_num}\"", "")
      f.write(line)


def remove_text(file_path, text, replace=""):
  with open(file_path, "r") as f:
    lines = f.readlines()
  with open(file_path, "w") as f:
    for line in lines:
      if text in line:
        line = line.replace(text, replace)
      f.write(line)


def empty_file(file):
  with open(file, "r+") as f:

    # Delete all lines from the file.
    f.truncate(0)

    # Close the file.
    f.close()


def cmake_pcre(init_directory):
  os.chdir("./httpd/srclib/pcre")

  cmd = [
      "cmake",
      "-G",
      "NMake Makefiles",
      "-DCMAKE_BUILD_TYPE=RelWithDebInfo",
      "-DBUILD_SHARED_LIBS=ON",
      "-DPCRE_BUILD_PCRECPP=OFF",
      "-DPCRE_BUILD_PCREGREP=OFF",
      "-DPCRE_SUPPORT_UTF=ON",
      "-DPCRE_SUPPORT_JIT=ON",
      "-DPCRE_SUPPORT_BSR_ANYCRLF=ON",
      "-DPCRE_SUPPORT_UNICODE_PROPERTIES=ON",
  ]

  res = subprocess.run(cmd)

  if (res.returncode != 0):
    print("cmake -G NMake Makefiles failed")
    sys.exit(0)

  res = subprocess.run(["nmake"])

  if (res.returncode != 0):
    print("nmake inside pcre failed")
    return sys.exit(res.returncode)

  os.chdir(init_directory)


def delete_old_windows_compiler():
  with open("./httpd/srclib/apr/include/apr.hw", "r") as f:
    lines = f.readlines()
  with open("./httpd/srclib/apr/include/apr.hw", "w") as f:
    for line in lines:
      if line.strip("\n") != "#define _WIN32_WINNT 0x0501":
        f.write(line)


def unzip_tar_file(tar_file, old_name, clean_name, location):
  file = tarfile.open(tar_file)
  file.extractall(location)
  file.close()
  os.rename(
      os.path.join(location, old_name), os.path.join(location, clean_name))


def unzip_zip_file(zip_file_name, location):
  # Open the zip file in read mode.
  with zipfile.ZipFile(zip_file_name, "r") as zip_file:
    # Extract all the files and folders from the zip file to the current directory.
    print("extracting file", zip_file_name)
    zip_file.extractall(location)
    print("finished extracting file")

  # Close the `ZipFile` object.
  zip_file.close()


def unzip_tar_files():
  unzip_tar_file(f"../httpd-{httpd_version}.tar.gz", f"httpd-{httpd_version}",
                 "httpd", ".")
  unzip_tar_file(f"../apr-{apr_version}.tar.gz", f"apr-{apr_version}", "apr",
                 "./httpd/srclib")
  unzip_tar_file(f"../apr-iconv-{apr_iconv_version}.tar.gz",
                 f"apr-iconv-{apr_iconv_version}", "apr-iconv",
                 "./httpd/srclib")
  unzip_tar_file(f"../apr-util-{apr_util_version}.tar.gz",
                 f"apr-util-{apr_util_version}", "apr-util", "./httpd/srclib")
  unzip_tar_file(f"../pcre-{pcre_version}.tar.gz", f"pcre-{pcre_version}",
                 "pcre", "./httpd/srclib")
  unzip_tar_file(f"../openssl-{openssl_version}.tar.gz",
                 f"openssl-{openssl_version}", "openssl", "./httpd/srclib")
  unzip_tar_file(f"../zlib-{zlib_version}.tar.gz", f"zlib-{zlib_version}",
                 "zlib", "./httpd/srclib")

  unzip_zip_file(f"../gawk-{gawk_version}-bin.zip", f"./gawk-{gawk_version}")


def add_awk_to_path():
  awk_path = os.getcwd() + f"\\gawk-{gawk_version}\\bin"
  os.environ["PATH"] = os.environ.get("PATH") + ";" + awk_path


def get_tools_prefix():
  path_text = os.environ.get("PATH")
  path_dirs = path_text.split(";")

  for path_dir in path_dirs:
    if path_dir.endswith("tools_prefix"):
      return path_dir


def add_mingw_to_path():
  tools_prefix = get_tools_prefix()
  mingw_path = tools_prefix + "\\mingw64\\bin"
  os.environ["PATH"] = os.environ.get("PATH") + ";" + mingw_path


def nmake_zlib(init_directory):
  print("starting zlib compilation")

  os.chdir("./httpd/srclib/zlib")

  if os.environ.get("_3PP_PLATFORM") == "windows-amd64":
    os.environ["CL"] = "/D_WIN32_WINNT#0x0600 /D \"WIN64\" /D \"_WIN64\""
    os.environ["_LINK_"] = "/MANIFEST /MACHINE:x64"
    print("setting x64 settings")
  elif os.environ.get("_3PP_PLATFORM") == "windows-arm64":
    os.environ["CL"] = "/D_WIN32_WINNT=0x0A600 /D \"ARM64\" /D \"_ARM64_\""
    os.environ["_LINK_"] = "/MANIFEST /MACHINE:arm64"

  remove_text("./win32/Makefile.msc", "-base:0x5A4C0000")
  res = os.system("nmake -f win32/Makefile.msc")

  print("zlib returncode = ", res)

  if (res != 0):
    sys.exit(res)

  os.chdir(init_directory)
  print("finishing zlib compilation")


def nmake_install(init_directory):
  os.chdir("./httpd")

  out_dir = sys.argv[1]
  out_dir = os.path.relpath(out_dir, ".")

  nmake_install_cmd = f"nmake -f Makefile.win installr INSTDIR=\"{out_dir}\""

  res = os.system(nmake_install_cmd)

  os.chdir(init_directory)

  if res != 0:
    return sys.exit(res)


def setup_env_variables():
  # assume we are in /bin inside php-sdk-binary-tools
  init_directory = os.getcwd()
  os.chdir("..")
  sdk_tools_dir = os.getcwd()

  php_sdk_bin_path = sdk_tools_dir + "\\bin"
  php_msys2_bin_path = sdk_tools_dir + "\\msys2\\usr\\bin"

  os.environ["PHP_SDK_VC_DIR"] = "C:\\b\\s\\w\\ir\\cache\\windows_sdk\\VC"
  os.environ[
      "PATH"] = php_sdk_bin_path + ";" + php_msys2_bin_path + ";" + os.environ.get(
          "PATH")

  os.chdir(init_directory)


def setup_custom_php_env():
  init_directory = os.getcwd()

  res = subprocess.run([
      "git", "clone", "--depth=1",
      "https://github.com/php/php-sdk-binary-tools.git"
  ])

  if (res.returncode != 0):
    print("git clone inside php failed")
    return sys.exit(res.returncode)
  os.chdir("./php-sdk-binary-tools")
  os.chdir("./bin")

  print("setting php vars")
  setup_env_variables()

  os.chdir(init_directory)


def nmake_php(out_dir):
  init_directory = os.getcwd()
  setup_custom_php_env()
  print("completed php setup")

  print("completed building php library deps")
  if os.environ.get("_3PP_PLATFORM") == "windows-arm64":
    unzip_tar_file(f"../php-{php_version}.tar.gz", f"php-{php_version}", "php",
                   ".")
    os.chdir("./php")

  res = os.system("buildconf")
  if res != 0:
    print("buildconf failed")
    return sys.exit(res)
  print("buildconf completed")
  os.system("configure --help")

  print("configure --help complete")

  res = os.system(
      f"configure --with-prefix={out_dir} --disable-all --enable-cli --enable-apache2-4handler --with-prefix={out_dir} --with-php-build={out_dir}"
  )

  if res != 0:
    print("configure failed = ", res)
    return sys.exit(res)
  print("configure.bat complete")

  print("starting nmake")
  res = subprocess.run(["nmake"])

  if (res.returncode != 0):
    print("nmake inside php failed")
    return sys.exit(res.returncode)
  else:
    print("finished nmake successfully")
    print("nmake return code = ", res.returncode)

  os.system(f"nmake install INSTDIR=\"{out_dir}\"")

  os.chdir(init_directory)


def main():

  os.mkdir("./src")
  os.chdir("./src")

  init_directory = os.getcwd()

  unzip_tar_files()

  add_mingw_to_path()
  add_awk_to_path()

  delete_old_windows_compiler()
  if os.environ.get("_3PP_PLATFORM") == "windows-arm64":
    remove_base("./httpd/srclib/apr/libapr.mak", "0x6EEC0000")
    remove_base("./httpd/srclib/apr-iconv/libapriconv.mak", "0x6EE50000")
    remove_base("./httpd/srclib/apr-util/libaprutil.mak", "0x6EE60000")
    remove_base("./httpd/srclib/apr-util/crypto/apr_crypto_nss.mak",
                "0x6F110000")
    remove_base("./httpd/srclib/apr-util/crypto/apr_crypto_openssl.mak",
                "0x6F100000")
    remove_base("./httpd/srclib/apr-util/dbd/apr_dbd_freetds.mak", "0x6EF60000")
    remove_base("./httpd/srclib/apr-util/dbd/apr_dbd_mysql.mak", "0x6EF50000")
    remove_base("./httpd/srclib/apr-util/dbd/apr_dbd_odbc.mak", "0x6EF00000")
    remove_base("./httpd/srclib/apr-util/dbd/apr_dbd_oracle.mak", "0x6EF40000")
    remove_base("./httpd/srclib/apr-util/dbd/apr_dbd_pgsql.mak", "0x6EF30000")
    remove_base("./httpd/srclib/apr-util/dbd/apr_dbd_sqlite2.mak", "0x6EF10000")
    remove_base("./httpd/srclib/apr-util/dbd/apr_dbd_sqlite3.mak", "0x6EF20000")
    remove_base("./httpd/srclib/apr-util/dbm/apr_dbm_db.mak", "0x6F000000")
    remove_base("./httpd/srclib/apr-util/dbm/apr_dbm_gdbm.mak", "0x6F010000")
    remove_base("./httpd/srclib/apr-util/ldap/apr_ldap.mak", "0x6EEB0000")

    empty_file("./httpd/srclib/apr-iconv/build/BaseAddr.ref")
    empty_file("./httpd/os/win32/BaseAddr.ref")

  # starting httpd
  cmake_pcre(init_directory)
  build_openssl(init_directory)

  enable_openssl_apr()

  nmake_zlib(init_directory)

  nmake_windows_builder(init_directory)

  print("starting nmake install")
  nmake_install(init_directory)

  # finishing httpd
  out_dir = sys.argv[1]

  # starting php
  nmake_php(out_dir)

  # finishing php

  # move relefant php dlls into correct spots
  os.chdir(out_dir)
  shutil.copyfile("php8ts.dll", ".\\bin\\php8ts.dll")
  shutil.copyfile("php8apache2_4.dll", ".\\modules\\php8apache2_4.dll")


if __name__ == '__main__':
  main()
