class Box:
    AMOUNT = 1


class Other:
    VALUE = 2


def use(b: Box, o: Other):
    return b.AMOUNT + o.VALUE + Box.AMOUNT + Other.VALUE
