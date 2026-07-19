from collections import ChainMap, OrderedDict
import collections


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_nested_inline():
    na: dict[str, list[A]] = {"k": [A()]}
    nb: dict[str, list[B]] = {"k": [B()]}
    return ChainMap(na)["k"][0].run() + ChainMap(nb)["k"][0].run()


def use_nested_assign():
    na: dict[str, list[A]] = {"k": [A()]}
    nb: dict[str, list[B]] = {"k": [B()]}
    ca = ChainMap(na)
    cb = ChainMap(nb)
    return ca["k"][0].run() + cb["k"][0].run()


def use_nested_for():
    na: dict[str, list[A]] = {"k": [A()]}
    n = 0
    for a in ChainMap(na)["k"]:
        n += a.run()
    return n


def use_nested_multi():
    na: dict[str, list[A]] = {"k": [A()]}
    pa: dict[str, list[A]] = {"j": [A()]}
    nb: dict[str, list[B]] = {"k": [B()]}
    pb: dict[str, list[B]] = {"j": [B()]}
    return ChainMap(na, pa)["j"][0].run() + ChainMap(nb, pb)["j"][0].run()


def use_nested_unannotated():
    na = {"k": [A()]}
    nb = {"k": [B()]}
    return ChainMap(na)["k"][0].run() + ChainMap(nb)["k"][0].run()


def use_nested_od():
    oa = OrderedDict(k=[A()])
    ob = OrderedDict(k=[B()])
    return ChainMap(oa)["k"][0].run() + ChainMap(ob)["k"][0].run()


def use_collections_chainmap():
    na: dict[str, list[A]] = {"k": [A()]}
    nb: dict[str, list[B]] = {"k": [B()]}
    return collections.ChainMap(na)["k"][0].run() + collections.ChainMap(nb)["k"][0].run()


def use_nested_values_for():
    na: dict[str, list[A]] = {"k": [A()]}
    nb: dict[str, list[B]] = {"k": [B()]}
    n = 0
    for ga in ChainMap(na).values():
        n += ga[0].run()
    for gb in ChainMap(nb).values():
        n += gb[0].run()
    return n


def use_scalar_inline():
    sa: dict[str, A] = {"k": A()}
    sb: dict[str, B] = {"k": B()}
    return ChainMap(sa)["k"].run() + ChainMap(sb)["k"].run()


def use_scalar_assign():
    sa: dict[str, A] = {"k": A()}
    sb: dict[str, B] = {"k": B()}
    ca = ChainMap(sa)
    cb = ChainMap(sb)
    return ca["k"].run() + cb["k"].run()


def use_preserves_b():
    nb: dict[str, list[B]] = {"k": [B()]}
    return ChainMap(nb)["k"][0].run()
