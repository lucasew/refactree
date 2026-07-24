from boltons import fileutils
from boltons.fileutils import open_file


def test_open():
    assert open_file("x") == "x"
    assert fileutils.open_file("y") == "y"
