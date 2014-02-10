/*	
	A simple LD_PRELOAD hack to let you read /etc/passwd from
	a different directory

	Compile:

	gcc -fPIC -static -shared -o getpwhack.so getpwhack.c -lc -ldl

	You can add -DDEBUG to see debug output.

	Usage:

	LD_PRELOAD=/path/to/getpwhack.so <command>
	
	eg:

	LD_PRELOAD=/home/cc/getpwhack.so sshd -D

*/
#include <stdio.h>
#include <stdlib.h>
#include <sys/types.h>
#include <string.h>
#include <dlfcn.h>
#include <pwd.h>

/*
   LIBC_NAME should be the name of the library that contains the real
   bind() and connect() library calls. On Linux this is libc, but on
   other OS's such as Solaris this would be the socket library
*/
#define LIBC_NAME	"libc.so.6"

#define YES	1
#define NO	0


struct passwd *
getpwnam(const char *name)
{
	void	*libc;
	struct passwd *	(*base_ptr)(const char *);
	struct passwd	*ret;
	int	passthru;
	char 	*bind_src;

#ifdef DEBUG
	fprintf(stderr, "getpwnam(%s) override\n", name);
#endif

	libc = dlopen(LIBC_NAME, RTLD_LAZY);

	if (!libc)
	{
		fprintf(stderr, "Unable to open libc!\n");
		exit(-1);
	}

	*(void **) (&base_ptr) = dlsym(libc, "getpwnam");

	if (!base_ptr)
	{
		fprintf(stderr, "Unable to locate getpwnam function in lib\n");
		exit(-1);
	}

	/* Call the original function */
	ret = (struct passwd *)(*base_ptr)(name);

	/* Clean up */
	dlclose(libc);

	return ret;
}

struct passwd *
getpwuid(uid_t uid)
{
	void	*libc;
	struct passwd *	(*base_ptr)(uid_t);
	struct passwd	*ret;
	int	passthru;
	char 	*bind_src;

#ifdef DEBUG
	fprintf(stderr, "getpwuid(%i) override\n", uid);
#endif

	libc = dlopen(LIBC_NAME, RTLD_LAZY);

	if (!libc)
	{
		fprintf(stderr, "Unable to open libc!\n");
		exit(-1);
	}

	*(void **) (&base_ptr) = dlsym(libc, "getpwuid");

	if (!base_ptr)
	{
		fprintf(stderr, "Unable to locate getpwuid function in lib\n");
		exit(-1);
	}

	/* Call the original function */
	ret = (struct passwd *)(*base_ptr)(uid);

	/* Clean up */
	dlclose(libc);

	return ret;
}