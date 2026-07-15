class Box:
    pass


class Other:
    pass


Box.tag = 1
Other.tag = 2


def main():
    return Box.tag + Other.tag
