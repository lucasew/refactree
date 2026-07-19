class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_scalar_rhs():
    sa: dict[str, A] = {"k": A()}
    sb: dict[str, B] = {"k": B()}
    return (sa | {"j": A()})["j"].run() + (sb | {"j": B()})["j"].run()


def use_scalar_locals():
    sa: dict[str, A] = {"k": A()}
    ta: dict[str, A] = {"j": A()}
    sb: dict[str, B] = {"k": B()}
    tb: dict[str, B] = {"j": B()}
    return (sa | ta)["j"].run() + (sb | tb)["j"].run()


def use_scalar_assign():
    sa: dict[str, A] = {"k": A()}
    ta: dict[str, A] = {"j": A()}
    sb: dict[str, B] = {"k": B()}
    tb: dict[str, B] = {"j": B()}
    ca = sa | ta
    cb = sb | tb
    return ca["j"].run() + cb["j"].run()


def use_scalar_empty():
    sa = {} | {"k": A()}
    sb = {} | {"k": B()}
    return sa["k"].run() + sb["k"].run()


def use_scalar_unannotated():
    sa = {"k": A()}
    ta = {"j": A()}
    sb = {"k": B()}
    tb = {"j": B()}
    return (sa | ta)["j"].run() + (sb | tb)["j"].run()


def use_scalar_values():
    sa: dict[str, A] = {"k": A()}
    ta: dict[str, A] = {"j": A()}
    sb: dict[str, B] = {"k": B()}
    tb: dict[str, B] = {"j": B()}
    return list((sa | ta).values())[0].run() + list((sb | tb).values())[0].run()


def use_nested_rhs():
    na: dict[str, list[A]] = {"k": [A()]}
    nb: dict[str, list[B]] = {"k": [B()]}
    return (na | {"j": [A()]})["j"][0].run() + (nb | {"j": [B()]})["j"][0].run()


def use_nested_locals():
    na: dict[str, list[A]] = {"k": [A()]}
    pa: dict[str, list[A]] = {"j": [A()]}
    nb: dict[str, list[B]] = {"k": [B()]}
    pb: dict[str, list[B]] = {"j": [B()]}
    return (na | pa)["j"][0].run() + (nb | pb)["j"][0].run()


def use_nested_assign():
    na: dict[str, list[A]] = {"k": [A()]}
    pa: dict[str, list[A]] = {"j": [A()]}
    nb: dict[str, list[B]] = {"k": [B()]}
    pb: dict[str, list[B]] = {"j": [B()]}
    ca = na | pa
    cb = nb | pb
    return ca["j"][0].run() + cb["j"][0].run()


def use_nested_unannotated():
    na = {"k": [A()]}
    pa = {"j": [A()]}
    nb = {"k": [B()]}
    pb = {"j": [B()]}
    return (na | pa)["j"][0].run() + (nb | pb)["j"][0].run()


def use_nested_for():
    na: dict[str, list[A]] = {"k": [A()]}
    pa: dict[str, list[A]] = {"j": [A()]}
    n = 0
    for a in (na | pa)["j"]:
        n += a.run()
    return n


def use_preserves_b_scalar():
    sb: dict[str, B] = {"k": B()}
    return (sb | {"j": B()})["j"].run()


def use_preserves_b_nested():
    nb: dict[str, list[B]] = {"k": [B()]}
    return (nb | {"j": [B()]})["j"][0].run()


def use_scalar_get():
    sa: dict[str, A] = {"k": A()}
    ta: dict[str, A] = {"j": A()}
    sb: dict[str, B] = {"k": B()}
    tb: dict[str, B] = {"j": B()}
    return (sa | ta).get("j").run() + (sb | tb).get("j").run()


def use_nested_get():
    na: dict[str, list[A]] = {"k": [A()]}
    pa: dict[str, list[A]] = {"j": [A()]}
    nb: dict[str, list[B]] = {"k": [B()]}
    pb: dict[str, list[B]] = {"j": [B()]}
    return (na | pa).get("j")[0].run() + (nb | pb).get("j")[0].run()
