class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_next_values():
    return (
        next(iter(dict.fromkeys(["k"], A()).values())).run()
        + next(iter(dict.fromkeys(["k"], B()).values())).run()
    )


def use_next_values_direct():
    return (
        next(dict.fromkeys(["k"], A()).values()).run()
        + next(dict.fromkeys(["k"], B()).values()).run()
    )


def use_next_assign():
    xa = next(iter(dict.fromkeys(["k"], A()).values()))
    xb = next(iter(dict.fromkeys(["k"], B()).values()))
    return xa.run() + xb.run()


def use_values_for():
    s = 0
    for x in dict.fromkeys(["k"], A()).values():
        s += x.run()
    for y in dict.fromkeys(["k"], B()).values():
        s += y.run()
    return s


def use_list_values_index():
    return (
        list(dict.fromkeys(["k"], A()).values())[0].run()
        + list(dict.fromkeys(["k"], B()).values())[0].run()
    )


def use_fromkeys_value_kw():
    return (
        next(iter(dict.fromkeys(["k"], value=A()).values())).run()
        + next(iter(dict.fromkeys(["k"], value=B()).values())).run()
    )


def use_local_then_values():
    da = dict.fromkeys(["k"], A())
    db = dict.fromkeys(["k"], B())
    return (
        next(iter(da.values())).run()
        + next(iter(db.values())).run()
    )



def use_min_values():
    return (
        min(dict.fromkeys(["k"], A()).values()).run()
        + min(dict.fromkeys(["k"], B()).values()).run()
    )


def use_max_values():
    return (
        max(dict.fromkeys(["k"], A()).values()).run()
        + max(dict.fromkeys(["k"], B()).values()).run()
    )

def use_preserves_b():
    return next(iter(dict.fromkeys(["k"], B()).values())).run()
