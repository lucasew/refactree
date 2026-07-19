from collections import UserList
import collections


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_userlist_literal():
    xs = UserList([A()])
    ys = UserList([B()])
    return xs[0].run() + ys[0].run()


def use_userlist_for():
    xs = UserList([A()])
    ys = UserList([B()])
    n = 0
    for a in xs:
        n += a.run()
    for b in ys:
        n += b.run()
    return n


def use_userlist_var():
    xs = UserList([A()])
    ys = UserList([B()])
    a = xs[0]
    b = ys[0]
    return a.run() + b.run()


def use_userlist_append():
    xs = UserList()
    ys = UserList()
    xs.append(A())
    ys.append(B())
    return xs[0].run() + ys[0].run()


def use_userlist_append_for():
    xs = UserList()
    ys = UserList()
    xs.append(A())
    ys.append(B())
    n = 0
    for a in xs:
        n += a.run()
    for b in ys:
        n += b.run()
    return n


def use_collections_userlist():
    xs = collections.UserList([A()])
    ys = collections.UserList([B()])
    return xs[0].run() + ys[0].run()


def use_preserves_b():
    ys = UserList([B()])
    return ys[0].run()
