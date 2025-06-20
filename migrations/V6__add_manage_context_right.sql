ALTER TYPE user_right ADD VALUE 'manage_context';

UPDATE xi_users 
SET rights = array_append(rights, 'manage_context'::user_right)
WHERE username = 'mairwunnx' 
  AND NOT ('manage_context'::user_right = ANY(rights)); 