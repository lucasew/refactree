def deco(f):
    return f


@deco
def helper():
    return 1


def main():
    return helper()
