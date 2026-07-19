class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_list_mul():
    aa = [A()] * 2
    bb = [B()] * 2
    return aa[0].run() + bb[0].run()


def use_tuple_mul():
    aa = (A(),) * 2
    bb = (B(),) * 2
    return aa[0].run() + bb[0].run()


def use_mul_left():
    aa = 2 * [A()]
    bb = 2 * [B()]
    return aa[0].run() + bb[0].run()


def use_mul_assign():
    aa = [A()] * 3
    bb = [B()] * 3
    xa = aa[0]
    xb = bb[0]
    return xa.run() + xb.run()
