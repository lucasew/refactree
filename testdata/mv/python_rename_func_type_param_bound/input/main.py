class Helper:
    pass


class Stay:
    pass


def use[T: Helper](x: T) -> T:
    return x


def other(x: Stay) -> Stay:
    return x
