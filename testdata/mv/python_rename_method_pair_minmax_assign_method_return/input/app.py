from random import choice
from heapq import heappop


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


class BoxA:
    def __init__(self, a: A):
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self, b: B):
        self.b = b

    def get(self) -> B:
        return self.b


def use_min_assign(ba: BoxA, bb: BoxB):
    pmin_a = min(list(zip([ba.get()], [ba.get()])))
    pmin_b = min(list(zip([bb.get()], [bb.get()])))
    return pmin_a[0].run() + pmin_b[0].run()


def use_max_assign(ba: BoxA, bb: BoxB):
    pmax_a = max(list(zip([ba.get()], [ba.get()])))
    pmax_b = max(list(zip([bb.get()], [bb.get()])))
    return pmax_a[0].run() + pmax_b[0].run()


def use_min_walrus(ba: BoxA, bb: BoxB):
    if (pwal_a := min(list(zip([ba.get()], [ba.get()])))):
        if (pwal_b := min(list(zip([bb.get()], [bb.get()])))):
            return pwal_a[0].run() + pwal_b[0].run()
    return 0


def use_min_unpack_from_assign(ba: BoxA, bb: BoxB):
    pup_a = min(list(zip([ba.get()], [ba.get()])))
    pup_b = min(list(zip([bb.get()], [bb.get()])))
    a, _ = pup_a
    b, _ = pup_b
    return a.run() + b.run()


def use_choice_assign(ba: BoxA, bb: BoxB):
    pch_a = choice(list(zip([ba.get()], [ba.get()])))
    pch_b = choice(list(zip([bb.get()], [bb.get()])))
    return pch_a[0].run() + pch_b[0].run()


def use_heappop_assign(ba: BoxA, bb: BoxB):
    php_a = heappop(list(zip([ba.get()], [ba.get()])))
    php_b = heappop(list(zip([bb.get()], [bb.get()])))
    return php_a[0].run() + php_b[0].run()


def use_direct(ba: BoxA, bb: BoxB):
    return (
        min(list(zip([ba.get()], [ba.get()])))[0].run()
        + min(list(zip([bb.get()], [bb.get()])))[0].run()
    )


def use_class():
    pcls_a = min(list(zip([A()], [A()])))
    pcls_b = min(list(zip([B()], [B()])))
    return pcls_a[0].run() + pcls_b[0].run()


def use_preserves_b(bb: BoxB):
    ppre_b = min(list(zip([bb.get()], [bb.get()])))
    return ppre_b[0].run()
