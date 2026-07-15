class Crate:
    pass


class Other:
    pass


Crate.tag = 1
Other.tag = 2


def main():
    return Crate.tag + Other.tag
