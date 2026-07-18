from operator import getitem
import operator


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_dunder_getitem(items: list[A]):
    a = items.__getitem__(0)
    a.run()


def use_dunder_getitem_b(items: list[B]):
    b = items.__getitem__(0)
    b.run()


def use_dunder_direct(items: list[A]):
    return items.__getitem__(0).run()


def use_operator_getitem(items: list[A]):
    a = operator.getitem(items, 0)
    a.run()


def use_operator_getitem_b(items: list[B]):
    b = operator.getitem(items, 0)
    b.run()


def use_operator_direct(items: list[A]):
    return operator.getitem(items, 0).run()


def use_from_import_getitem(items: list[A]):
    a = getitem(items, 0)
    a.run()


def use_from_import_direct(items: list[A]):
    return getitem(items, 0).run()


def use_dunder_walrus(items: list[A]):
    if (a := items.__getitem__(0)):
        a.run()


def use_operator_walrus(items: list[A]):
    if (a := operator.getitem(items, 0)):
        a.run()


def use_wrapper(items: list[A]):
    a = list(items).__getitem__(0)
    a.run()


def use_dict(d: dict[str, A]):
    a = d.__getitem__("k")
    a.run()


def use_dict_op(d: dict[str, A]):
    a = operator.getitem(d, "k")
    a.run()


def use_assigned_literal():
    xs = [A()]
    a = xs.__getitem__(0)
    a.run()
    ys = [B()]
    b = ys.__getitem__(0)
    b.run()
