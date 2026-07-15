class Assist:
    pass


class Stay:
    pass


def use[T: Assist](x: T) -> T:
    return x


def other(x: Stay) -> Stay:
    return x
