from types import SimpleNamespace
import types


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_sns_match():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    match da:
        case SimpleNamespace(k=xa):
            r = xa.run()
    match db:
        case SimpleNamespace(k=xb):
            r += xb.run()
    return r


def use_sns_match_as():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    match da:
        case SimpleNamespace(k=xa as x):
            r = x.run()
    match db:
        case SimpleNamespace(k=xb as y):
            r += y.run()
    return r


def use_sns_multi_match():
    da = SimpleNamespace(k=A(), m=A())
    db = SimpleNamespace(k=B(), m=B())
    match da:
        case SimpleNamespace(k=xa, m=ma):
            r = xa.run() + ma.run()
    match db:
        case SimpleNamespace(k=xb, m=mb):
            r += xb.run() + mb.run()
    return r


def use_types_sns_match():
    da = types.SimpleNamespace(k=A())
    db = types.SimpleNamespace(k=B())
    match da:
        case types.SimpleNamespace(k=xa):
            r = xa.run()
    match db:
        case types.SimpleNamespace(k=xb):
            r += xb.run()
    return r


def use_preserves_b():
    db = SimpleNamespace(k=B())
    match db:
        case SimpleNamespace(k=xb):
            return xb.run()
    return 0
