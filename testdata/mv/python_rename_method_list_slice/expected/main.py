class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_slice():
    as_: list[A] = [A()]
    bs: list[B] = [B()]
    return as_[:1][0].execute() + bs[:1][0].run()


def use_slice_mid():
    as_: list[A] = [A(), A()]
    bs: list[B] = [B(), B()]
    return as_[0:1][0].execute() + bs[0:1][0].run()


def use_slice_full():
    as_: list[A] = [A()]
    bs: list[B] = [B()]
    return as_[:][0].execute() + bs[:][0].run()


def use_slice_local():
    as_: list[A] = [A()]
    bs: list[B] = [B()]
    sa = as_[:1]
    sb = bs[:1]
    return sa[0].execute() + sb[0].run()


def use_preserves_b():
    bs: list[B] = [B()]
    return bs[:1][0].run()
