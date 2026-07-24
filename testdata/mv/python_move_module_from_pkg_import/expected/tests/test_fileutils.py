from boltons import fileutils_fuzz as fileutils
from boltons.fileutils_fuzz import open_file


def test_open():
    assert open_file("x") == "x"
    assert fileutils.open_file("y") == "y"
