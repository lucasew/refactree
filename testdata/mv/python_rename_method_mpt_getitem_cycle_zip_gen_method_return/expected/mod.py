from types import MappingProxyType
from operator import getitem
import operator
import itertools
from itertools import cycle, chain


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    a: A

    def __init__(self, a: A):
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    b: B

    def __init__(self, b: B):
        self.b = b

    def get(self) -> B:
        return self.b


def use_mpt_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        MappingProxyType({"k": ba.get()})["k"].execute()
        + MappingProxyType({"k": bb.get()})["k"].run()
    )


def use_mpt_get(ba: BoxA, bb: BoxB) -> int:
    return (
        MappingProxyType({"k": ba.get()}).get("k").execute()
        + MappingProxyType({"k": bb.get()}).get("k").run()
    )


def use_mpt_values(ba: BoxA, bb: BoxB) -> int:
    return (
        list(MappingProxyType({"k": ba.get()}).values())[0].execute()
        + list(MappingProxyType({"k": bb.get()}).values())[0].run()
    )


def use_mpt_assign(ba: BoxA, bb: BoxB) -> int:
    mpa = MappingProxyType({"k": ba.get()})
    mpb = MappingProxyType({"k": bb.get()})
    return mpa["k"].execute() + mpb["k"].run()


def use_getitem_op(ba: BoxA, bb: BoxB) -> int:
    return (
        operator.getitem([ba.get()], 0).execute()
        + operator.getitem([bb.get()], 0).run()
    )


def use_getitem_bare(ba: BoxA, bb: BoxB) -> int:
    return getitem([ba.get()], 0).execute() + getitem([bb.get()], 0).run()


def use_getitem_assign(ba: BoxA, bb: BoxB) -> int:
    opa = operator.getitem([ba.get()], 0)
    opb = getitem([bb.get()], 0)
    dua = [ba.get()].__getitem__(0)
    dub = [bb.get()].__getitem__(0)
    return opa.execute() + opb.run() + dua.execute() + dub.run()


def use_dunder_direct(ba: BoxA, bb: BoxB) -> int:
    return (
        [ba.get()].__getitem__(0).execute()
        + [bb.get()].__getitem__(0).run()
    )


def use_cycle(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for cyc_a in cycle([ba.get()]):
        n += cyc_a.execute()
        break
    for cyc_b in itertools.cycle([bb.get()]):
        n += cyc_b.run()
        break
    return n


def use_chain(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for ch_a in chain([ba.get()]):
        n += ch_a.execute()
    for ch_b in itertools.chain([bb.get()]):
        n += ch_b.run()
    return n


def use_zip(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for zip_a, _ in zip([ba.get()], [0]):
        n += zip_a.execute()
    for zip_b, _ in zip([bb.get()], [0]):
        n += zip_b.run()
    return n


def use_zip_next(ba: BoxA, bb: BoxB) -> int:
    nxa, _ = next(zip([ba.get()], [0]))
    nxb, _ = next(zip([bb.get()], [0]))
    return nxa.execute() + nxb.run()


def use_enum(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for i, enum_a in enumerate([ba.get()]):
        n += enum_a.execute()
    for i, enum_b in enumerate([bb.get()]):
        n += enum_b.run()
    return n


def use_enum_sub(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for item_a in enumerate([ba.get()]):
        n += item_a[1].execute()
    for item_b in enumerate([bb.get()]):
        n += item_b[1].run()
    return n


def use_gen_direct(ba: BoxA, bb: BoxB) -> int:
    return (
        next(x for x in [ba.get()]).execute()
        + next(x for x in [bb.get()]).run()
    )


def use_gen_assign(ba: BoxA, bb: BoxB) -> int:
    mra = next(x for x in [ba.get()])
    mrb = next(x for x in [bb.get()])
    return mra.execute() + mrb.run()


def use_preserves_b(bb: BoxB) -> int:
    n = 0
    for cyc_b in cycle([bb.get()]):
        n += cyc_b.run()
        break
    mpb = MappingProxyType({"k": bb.get()})
    return (
        n
        + mpb["k"].run()
        + operator.getitem([bb.get()], 0).run()
        + next(x for x in [bb.get()]).run()
    )
