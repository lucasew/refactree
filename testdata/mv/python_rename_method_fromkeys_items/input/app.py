class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_fromkeys_items_next():
    return (
        next(iter(dict.fromkeys(["k"], A()).items()))[1].run()
        + next(iter(dict.fromkeys(["k"], B()).items()))[1].run()
    )


def use_fromkeys_items_for():
    n = 0
    for _, va in dict.fromkeys(["k"], A()).items():
        n += va.run()
    for _, vb in dict.fromkeys(["k"], B()).items():
        n += vb.run()
    return n


def use_fromkeys_items_list():
    return (
        list(dict.fromkeys(["k"], A()).items())[0][1].run()
        + list(dict.fromkeys(["k"], B()).items())[0][1].run()
    )


def use_fromkeys_value_kw():
    return (
        next(iter(dict.fromkeys(["k"], value=A()).items()))[1].run()
        + next(iter(dict.fromkeys(["k"], value=B()).items()))[1].run()
    )


def use_preserves_b():
    return next(iter(dict.fromkeys(["k"], B()).items()))[1].run()
