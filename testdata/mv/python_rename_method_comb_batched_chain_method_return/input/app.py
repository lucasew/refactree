from itertools import combinations, permutations, combinations_with_replacement, batched, chain
import itertools


class A:
    def run(self) -> int:
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


def use_comb_direct(ba: BoxA, bb: BoxB) -> int:
    return (
        list(combinations([ba.get(), ba.get()], 2))[0][0].run()
        + list(combinations([bb.get(), bb.get()], 2))[0][0].run()
    )


def use_comb_assign(ba: BoxA, bb: BoxB) -> int:
    xa = list(combinations([ba.get(), ba.get()], 2))[0][0]
    xb = list(combinations([bb.get(), bb.get()], 2))[0][0]
    return xa.run() + xb.run()


def use_perm_direct(ba: BoxA, bb: BoxB) -> int:
    return (
        list(permutations([ba.get(), ba.get()], 2))[0][0].run()
        + list(permutations([bb.get(), bb.get()], 2))[0][0].run()
    )


def use_cwr_direct(ba: BoxA, bb: BoxB) -> int:
    return (
        list(combinations_with_replacement([ba.get()], 2))[0][0].run()
        + list(combinations_with_replacement([bb.get()], 2))[0][0].run()
    )


def use_batched_direct(ba: BoxA, bb: BoxB) -> int:
    return (
        list(batched([ba.get(), ba.get()], 2))[0][0].run()
        + list(batched([bb.get(), bb.get()], 2))[0][0].run()
    )


def use_batched_itertools(ba: BoxA, bb: BoxB) -> int:
    return (
        list(itertools.batched([ba.get(), ba.get()], 2))[0][0].run()
        + list(itertools.batched([bb.get(), bb.get()], 2))[0][0].run()
    )


def use_chain_from_iterable(ba: BoxA, bb: BoxB) -> int:
    return (
        list(chain.from_iterable([[ba.get()]]))[0].run()
        + list(chain.from_iterable([[bb.get()]]))[0].run()
    )


def use_chain_from_iterable_itertools(ba: BoxA, bb: BoxB) -> int:
    return (
        list(itertools.chain.from_iterable([[ba.get()]]))[0].run()
        + list(itertools.chain.from_iterable([[bb.get()]]))[0].run()
    )


def use_comb_class() -> int:
    return (
        list(combinations([A(), A()], 2))[0][0].run()
        + list(combinations([B(), B()], 2))[0][0].run()
        + list(batched([A(), A()], 2))[0][0].run()
        + list(chain.from_iterable([[A()]]))[0].run()
        + list(chain.from_iterable([[B()]]))[0].run()
    )


def use_preserves_b(bb: BoxB) -> int:
    return (
        list(combinations([bb.get(), bb.get()], 2))[0][0].run()
        + list(permutations([bb.get(), bb.get()], 2))[0][0].run()
        + list(batched([bb.get(), bb.get()], 2))[0][0].run()
        + list(chain.from_iterable([[bb.get()]]))[0].run()
    )
