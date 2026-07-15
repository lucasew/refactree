class Box:
    VALUE = 1


class Other:
    VALUE = 2


def use(b: Box, o: Other):
    return b.VALUE + o.VALUE + Box.VALUE + Other.VALUE
